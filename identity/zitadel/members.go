package zitadel

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"connectrpc.com/connect"
	authorizationv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/authorization/v2"
	filterv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/filter/v2"
	userv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

// ListMembers returns the tenant's users joined with the filament role each
// holds. Both reads are scoped to the caller's own organization.
func (p *Provider) ListMembers(ctx context.Context, _ *connect.Request[authv1.ListMembersRequest]) (*connect.Response[authv1.ListMembersResponse], error) {
	caller, ok := identity.CallerFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	orgID := caller.TenantExternalID

	users, err := p.api.UserServiceV2().ListUsers(ctx, &userv2.ListUsersRequest{
		Queries: []*userv2.SearchQuery{
			{Query: &userv2.SearchQuery_OrganizationIdQuery{
				OrganizationIdQuery: &userv2.OrganizationIdQuery{OrganizationId: orgID},
			}},
			{Query: &userv2.SearchQuery_TypeQuery{
				TypeQuery: &userv2.TypeQuery{Type: userv2.Type_TYPE_HUMAN},
			}},
		},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	roles, err := p.rolesByUser(ctx, orgID)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}

	members := make([]*authv1.Member, 0, len(users.GetResult()))
	for _, user := range users.GetResult() {
		human := user.GetHuman()
		name := strings.TrimSpace(human.GetProfile().GetGivenName() + " " + human.GetProfile().GetFamilyName())
		members = append(members, &authv1.Member{
			UserId: user.GetUserId(),
			Name:   name,
			Email:  human.GetEmail().GetEmail(),
			Role:   roles[user.GetUserId()],
		})
	}
	return connect.NewResponse(&authv1.ListMembersResponse{
		Members:   members,
		CanManage: caller.IsAdmin(),
	}), nil
}

// rolesByUser maps each user in the organization to the filament role their
// authorization grants.
func (p *Provider) rolesByUser(ctx context.Context, orgID string) (map[string]authv1.Role, error) {
	granted, err := p.api.AuthorizationServiceV2().ListAuthorizations(ctx, &authorizationv2.ListAuthorizationsRequest{
		Filters: []*authorizationv2.AuthorizationsSearchFilter{
			{Filter: &authorizationv2.AuthorizationsSearchFilter_ProjectId{ProjectId: &filterv2.IDFilter{Id: p.project.id}}},
			{Filter: &authorizationv2.AuthorizationsSearchFilter_OrganizationId{OrganizationId: &filterv2.IDFilter{Id: orgID}}},
		},
	})
	if err != nil {
		return nil, err
	}
	roles := make(map[string]authv1.Role, len(granted.GetAuthorizations()))
	for _, authorization := range granted.GetAuthorizations() {
		if role := grantedRole(authorization); role != authv1.Role_ROLE_UNSPECIFIED {
			roles[authorization.GetUser().GetId()] = role
		}
	}
	return roles, nil
}

// grantedRole is the filament role an authorization carries, unspecified
// when none of its keys is one filament recognizes.
func grantedRole(authorization *authorizationv2.Authorization) authv1.Role {
	for _, key := range authorization.GetRoles() {
		if role := identity.RoleFromKey(key.GetKey()); role != authv1.Role_ROLE_UNSPECIFIED {
			return role
		}
	}
	return authv1.Role_ROLE_UNSPECIFIED
}

// InviteMember creates a teammate in the caller's tenant and returns the
// code that redeems their invitation. Nothing is emailed; the admin shares
// the link.
func (p *Provider) InviteMember(ctx context.Context, req *connect.Request[authv1.InviteMemberRequest]) (*connect.Response[authv1.InviteMemberResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if m.GetEmail() == "" || m.GetGivenName() == "" || m.GetFamilyName() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("email, given_name, and family_name are required"))
	}
	roleKey := identity.RoleKey(m.GetRole())
	if roleKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}
	orgID := caller.TenantExternalID

	created, err := p.api.UserServiceV2().CreateUser(ctx, &userv2.CreateUserRequest{
		OrganizationId: orgID,
		UserType: &userv2.CreateUserRequest_Human_{
			Human: &userv2.CreateUserRequest_Human{
				Profile: &userv2.SetHumanProfile{
					GivenName:  m.GetGivenName(),
					FamilyName: m.GetFamilyName(),
				},
				Email: &userv2.SetHumanEmail{Email: m.GetEmail()},
			},
		},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	// returnCode hands the invitation back instead of mailing it.
	invite, err := p.api.UserServiceV2().CreateInviteCode(ctx, &userv2.CreateInviteCodeRequest{
		UserId:       created.GetId(),
		Verification: &userv2.CreateInviteCodeRequest_ReturnCode{ReturnCode: &userv2.ReturnInviteCode{}},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, p.discardUser(ctx, created.GetId(), err))
	}
	if err := p.grantRole(ctx, orgID, created.GetId(), roleKey); err != nil {
		return nil, connect.NewError(connect.CodeInternal, p.discardUser(ctx, created.GetId(), err))
	}
	return connect.NewResponse(&authv1.InviteMemberResponse{
		UserId: created.GetId(),
		Code:   invite.GetInviteCode(),
	}), nil
}

// discardUser deletes a user whose invitation did not complete, so the email
// is free for a retry rather than locked behind a half-created account. It
// runs detached from the request so a dropped client still triggers cleanup.
// cause is returned as-is on success; a failed delete is joined onto it, since
// the admin then has to remove the user by hand.
func (p *Provider) discardUser(ctx context.Context, userID string, cause error) error {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if _, err := p.api.UserServiceV2().DeleteUser(ctx, &userv2.DeleteUserRequest{UserId: userID}); err != nil {
		return errors.Join(cause, fmt.Errorf("cleanup of user %s failed, remove it manually: %w", userID, err))
	}
	return cause
}

// SetMemberRole reassigns a member's role, creating the authorization when
// the member has none yet.
func (p *Provider) SetMemberRole(ctx context.Context, req *connect.Request[authv1.SetMemberRoleRequest]) (*connect.Response[authv1.SetMemberRoleResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	m := req.Msg
	if m.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	roleKey := identity.RoleKey(m.GetRole())
	if roleKey == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("role is required"))
	}
	orgID := caller.TenantExternalID

	// Scoping the lookup to the caller's organization is what keeps one
	// tenant from re-roling another's members.
	existing, err := p.api.AuthorizationServiceV2().ListAuthorizations(ctx, &authorizationv2.ListAuthorizationsRequest{
		Filters: []*authorizationv2.AuthorizationsSearchFilter{
			{Filter: &authorizationv2.AuthorizationsSearchFilter_ProjectId{ProjectId: &filterv2.IDFilter{Id: p.project.id}}},
			{Filter: &authorizationv2.AuthorizationsSearchFilter_OrganizationId{OrganizationId: &filterv2.IDFilter{Id: orgID}}},
			{Filter: &authorizationv2.AuthorizationsSearchFilter_InUserIds{InUserIds: &filterv2.InIDsFilter{Ids: []string{m.GetUserId()}}}},
		},
	})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	if found := existing.GetAuthorizations(); len(found) > 0 {
		if _, err := p.api.AuthorizationServiceV2().UpdateAuthorization(ctx, &authorizationv2.UpdateAuthorizationRequest{
			Id:       found[0].GetId(),
			RoleKeys: []string{roleKey},
		}); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		return connect.NewResponse(&authv1.SetMemberRoleResponse{}), nil
	}
	if err := p.grantRole(ctx, orgID, m.GetUserId(), roleKey); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&authv1.SetMemberRoleResponse{}), nil
}

// RemoveMember deletes a member of the caller's tenant. Callers cannot
// remove themselves, and members of other tenants are unreachable.
func (p *Provider) RemoveMember(ctx context.Context, req *connect.Request[authv1.RemoveMemberRequest]) (*connect.Response[authv1.RemoveMemberResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	userID := req.Msg.GetUserId()
	if userID == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	if userID == caller.UserID {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("you cannot remove yourself"))
	}
	target, err := p.api.UserServiceV2().GetUserByID(ctx, &userv2.GetUserByIDRequest{UserId: userID})
	if err != nil {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("member not found"))
	}
	if target.GetUser().GetDetails().GetResourceOwner() != caller.TenantExternalID {
		return nil, connect.NewError(connect.CodeNotFound, errors.New("member not found"))
	}
	if _, err := p.api.UserServiceV2().DeleteUser(ctx, &userv2.DeleteUserRequest{UserId: userID}); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	return connect.NewResponse(&authv1.RemoveMemberResponse{}), nil
}
