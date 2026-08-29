package zitadel

import (
	"context"
	"errors"
	"strings"

	"connectrpc.com/connect"
	filterv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/filter/v2"
	metadatav2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/metadata/v2"
	userv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

const managedServiceAccountKey = "filament.service_account"

// ListServiceAccounts returns the machine users in the caller's organization.
// Client secrets are never readable after creation and are not part of the
// response.
func (p *Provider) ListServiceAccounts(ctx context.Context, _ *connect.Request[authv1.ListServiceAccountsRequest]) (*connect.Response[authv1.ListServiceAccountsResponse], error) {
	caller, ok := identity.CallerFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	orgID := caller.TenantExternalID
	users, err := p.api.UserServiceV2().ListUsers(ctx, &userv2.ListUsersRequest{
		Queries: managedServiceAccountQueries(orgID),
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	roles, err := p.rolesByUser(ctx, orgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	accounts := make([]*authv1.ServiceAccount, 0, len(users.GetResult()))
	for _, user := range users.GetResult() {
		machine := user.GetMachine()
		accounts = append(accounts, &authv1.ServiceAccount{
			UserId:      user.GetUserId(),
			ClientId:    user.GetUserId(),
			Name:        machine.GetName(),
			Description: machine.GetDescription(),
			Role:        roles[user.GetUserId()],
		})
	}
	return connect.NewResponse(&authv1.ListServiceAccountsResponse{
		ServiceAccounts: accounts,
		CanManage:       caller.IsAdmin(),
	}), nil
}

// CreateServiceAccount creates a JWT machine user, grants its filament role,
// and returns the client secret exactly once.
func (p *Provider) CreateServiceAccount(ctx context.Context, req *connect.Request[authv1.CreateServiceAccountRequest]) (*connect.Response[authv1.CreateServiceAccountResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	name := strings.TrimSpace(req.Msg.GetName())
	if name == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("name is required"))
	}
	roleKey := identity.RoleKey(req.Msg.GetRole())
	if roleKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}
	description := strings.TrimSpace(req.Msg.GetDescription())
	created, err := p.api.UserServiceV2().CreateUser(ctx, &userv2.CreateUserRequest{
		OrganizationId: caller.TenantExternalID,
		Metadata: []*userv2.Metadata{{
			Key:   managedServiceAccountKey,
			Value: []byte("true"),
		}},
		UserType: &userv2.CreateUserRequest_Machine_{
			Machine: &userv2.CreateUserRequest_Machine{
				Name:            name,
				Description:     &description,
				AccessTokenType: userv2.AccessTokenType_ACCESS_TOKEN_TYPE_JWT,
			},
		},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	userID := created.GetId()
	cleanup := func() {
		_, _ = p.api.UserServiceV2().DeleteUser(ctx, &userv2.DeleteUserRequest{UserId: userID})
	}
	secret, err := p.api.UserServiceV2().AddSecret(ctx, &userv2.AddSecretRequest{UserId: userID})
	if err != nil {
		cleanup()
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if err := p.grantRole(ctx, caller.TenantExternalID, userID, roleKey); err != nil {
		cleanup()
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&authv1.CreateServiceAccountResponse{
		ServiceAccount: &authv1.ServiceAccount{
			UserId:      userID,
			ClientId:    userID,
			Name:        name,
			Description: description,
			Role:        req.Msg.GetRole(),
		},
		ClientSecret: secret.GetClientSecret(),
	}), nil
}

// RotateServiceAccountSecret invalidates the current client secret and returns
// its replacement exactly once.
func (p *Provider) RotateServiceAccountSecret(ctx context.Context, req *connect.Request[authv1.RotateServiceAccountSecretRequest]) (*connect.Response[authv1.RotateServiceAccountSecretResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	if err := p.requireServiceAccount(ctx, caller.TenantExternalID, userID); err != nil {
		return nil, err
	}
	if _, err := p.api.UserServiceV2().RemoveSecret(ctx, &userv2.RemoveSecretRequest{UserId: userID}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	secret, err := p.api.UserServiceV2().AddSecret(ctx, &userv2.AddSecretRequest{UserId: userID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&authv1.RotateServiceAccountSecretResponse{
		ClientId:     userID,
		ClientSecret: secret.GetClientSecret(),
	}), nil
}

// RemoveServiceAccount deletes a machine user in the caller's organization.
func (p *Provider) RemoveServiceAccount(ctx context.Context, req *connect.Request[authv1.RemoveServiceAccountRequest]) (*connect.Response[authv1.RemoveServiceAccountResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	if err := p.requireServiceAccount(ctx, caller.TenantExternalID, userID); err != nil {
		return nil, err
	}
	if _, err := p.api.UserServiceV2().DeleteUser(ctx, &userv2.DeleteUserRequest{UserId: userID}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&authv1.RemoveServiceAccountResponse{}), nil
}

func (p *Provider) requireServiceAccount(ctx context.Context, orgID, userID string) error {
	users, err := p.api.UserServiceV2().ListUsers(ctx, &userv2.ListUsersRequest{
		Queries: managedServiceAccountQueries(orgID, userID),
	})
	if err != nil {
		return connect.NewError(connect.CodeInternal, err)
	}
	if len(users.GetResult()) == 0 {
		return connect.NewError(connect.CodeNotFound, errors.New("service account not found"))
	}
	return nil
}

// managedServiceAccountQueries excludes Zitadel's own bootstrap/admin machine
// users. Only identities created by Filament carry this marker and can be
// listed, rotated, or removed through the application.
func managedServiceAccountQueries(orgID string, userIDs ...string) []*userv2.SearchQuery {
	queries := []*userv2.SearchQuery{
		{Query: &userv2.SearchQuery_OrganizationIdQuery{
			OrganizationIdQuery: &userv2.OrganizationIdQuery{OrganizationId: orgID},
		}},
		{Query: &userv2.SearchQuery_TypeQuery{
			TypeQuery: &userv2.TypeQuery{Type: userv2.Type_TYPE_MACHINE},
		}},
		{Query: &userv2.SearchQuery_MetadataKeyFilter{
			MetadataKeyFilter: &metadatav2.MetadataKeyFilter{
				Key:    managedServiceAccountKey,
				Method: filterv2.TextFilterMethod_TEXT_FILTER_METHOD_EQUALS,
			},
		}},
	}
	if len(userIDs) > 0 {
		queries = append(queries, &userv2.SearchQuery{Query: &userv2.SearchQuery_InUserIdsQuery{
			InUserIdsQuery: &userv2.InUserIDQuery{UserIds: userIDs},
		}})
	}
	return queries
}
