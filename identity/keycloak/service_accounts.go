package keycloak

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"strings"

	"connectrpc.com/connect"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

// A service account is a confidential client of its own, minting
// client-credentials tokens. Its service-account user is a member of the
// tenant and holds the filament role, so its tokens read like a person's.
// Attributes mark the clients filament manages and the tenant each belongs
// to.
const (
	serviceAccountClientPrefix = "filament-sa-"
	managedKey                 = "filament.service_account"
	tenantKey                  = "filament.tenant"
)

// GetToken runs the client-credentials grant for a service account. Proxying
// it keeps the scopes and the issuer the server's business, so a client only
// ever holds its id and secret.
func (p *Provider) GetToken(ctx context.Context, req *connect.Request[authv1.GetTokenRequest]) (*connect.Response[authv1.GetTokenResponse], error) {
	m := req.Msg
	if m.GetClientId() == "" || m.GetClientSecret() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("client_id and client_secret are required"))
	}
	// The organization scope is what puts the tenant on the token; Keycloak
	// never grants it by default.
	token, err := p.grantAs(ctx, m.GetClientId(), m.GetClientSecret(), url.Values{
		"grant_type": {"client_credentials"},
		"scope":      {"openid organization"},
	})
	if errors.Is(err, errGrantRejected) {
		// Unknown clients and wrong secrets both land here; neither should
		// tell the caller which it was.
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&authv1.GetTokenResponse{
		AccessToken: token.AccessToken,
		ExpiresIn:   token.ExpiresIn,
	}), nil
}

// managedServiceAccounts lists the clients filament manages for a tenant.
func (p *Provider) managedServiceAccounts(ctx context.Context, tenant string) ([]clientRep, error) {
	found, err := p.admin.clients(ctx)
	if err != nil {
		return nil, err
	}
	var own []clientRep
	for _, client := range found {
		if client.Attributes[managedKey] == "true" && client.Attributes[tenantKey] == tenant {
			own = append(own, client)
		}
	}
	return own, nil
}

// ListServiceAccounts returns the tenant's service accounts. Client secrets
// are only readable by an admin of the tenant through create and rotate and
// are not part of the response.
func (p *Provider) ListServiceAccounts(ctx context.Context, _ *connect.Request[authv1.ListServiceAccountsRequest]) (*connect.Response[authv1.ListServiceAccountsResponse], error) {
	caller, ok := identity.CallerFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	clients, err := p.managedServiceAccounts(ctx, caller.TenantExternalID)
	if err != nil {
		return nil, rpcError(err)
	}
	roles, err := p.rolesByUser(ctx)
	if err != nil {
		return nil, rpcError(err)
	}
	accounts := make([]*authv1.ServiceAccount, 0, len(clients))
	for _, client := range clients {
		user, err := p.admin.serviceAccountUser(ctx, client.ID)
		if err != nil {
			return nil, rpcError(err)
		}
		accounts = append(accounts, &authv1.ServiceAccount{
			UserId:      user.ID,
			ClientId:    client.ClientID,
			Name:        client.Name,
			Description: client.Description,
			Role:        roles[user.ID],
		})
	}
	return connect.NewResponse(&authv1.ListServiceAccountsResponse{
		ServiceAccounts: accounts,
		CanManage:       caller.IsAdmin(),
	}), nil
}

// CreateServiceAccount creates a client for the account, puts its user in
// the tenant with the filament role, and returns the client secret exactly
// once.
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
	org, err := p.tenant(ctx, caller)
	if err != nil {
		return nil, err
	}
	var raw [8]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	description := strings.TrimSpace(req.Msg.GetDescription())
	clientID := serviceAccountClientPrefix + hex.EncodeToString(raw[:])
	id, err := p.admin.createClient(ctx, clientRep{
		ClientID:               clientID,
		Name:                   name,
		Description:            description,
		ServiceAccountsEnabled: true,
		Attributes:             map[string]string{managedKey: "true", tenantKey: caller.TenantExternalID},
	})
	if err != nil {
		return nil, rpcError(err)
	}
	cleanup := func() { _ = p.admin.deleteClient(ctx, id) }
	if err := p.ensureTokenShape(ctx, id); err != nil {
		cleanup()
		return nil, rpcError(err)
	}
	user, err := p.admin.serviceAccountUser(ctx, id)
	if err != nil {
		cleanup()
		return nil, rpcError(err)
	}
	if err := p.admin.addMember(ctx, org.ID, user.ID); err != nil {
		cleanup()
		return nil, rpcError(err)
	}
	if err := p.grantRole(ctx, user.ID, roleKey); err != nil {
		cleanup()
		return nil, rpcError(err)
	}
	secret, err := p.admin.clientSecret(ctx, id)
	if err != nil {
		cleanup()
		return nil, rpcError(err)
	}
	return connect.NewResponse(&authv1.CreateServiceAccountResponse{
		ServiceAccount: &authv1.ServiceAccount{
			UserId:      user.ID,
			ClientId:    clientID,
			Name:        name,
			Description: description,
			Role:        req.Msg.GetRole(),
		},
		ClientSecret: secret,
	}), nil
}

// requireServiceAccount resolves the client behind a service account's user
// id, refusing users that are not filament-managed accounts of the caller's
// tenant.
func (p *Provider) requireServiceAccount(ctx context.Context, tenant, userID string) (clientRep, error) {
	notFound := connect.NewError(connect.CodeNotFound, errors.New("service account not found"))
	user, err := p.admin.getUser(ctx, userID)
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return clientRep{}, connect.NewError(connect.CodeUnavailable, err)
		}
		return clientRep{}, notFound
	}
	if !isServiceAccount(user) {
		return clientRep{}, notFound
	}
	client, err := p.admin.clientByClientID(ctx, strings.TrimPrefix(user.Username, serviceAccountPrefix))
	if err != nil {
		return clientRep{}, rpcError(err)
	}
	if client.Attributes[managedKey] != "true" || client.Attributes[tenantKey] != tenant {
		return clientRep{}, notFound
	}
	return client, nil
}

// RotateServiceAccountSecret invalidates the current client secret and returns
// its replacement exactly once.
func (p *Provider) RotateServiceAccountSecret(ctx context.Context, req *connect.Request[authv1.RotateServiceAccountSecretRequest]) (*connect.Response[authv1.RotateServiceAccountSecretResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	client, err := p.requireServiceAccount(ctx, caller.TenantExternalID, req.Msg.GetUserId())
	if err != nil {
		return nil, err
	}
	secret, err := p.admin.regenerateClientSecret(ctx, client.ID)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&authv1.RotateServiceAccountSecretResponse{
		ClientId:     client.ClientID,
		ClientSecret: secret,
	}), nil
}

// RemoveServiceAccount deletes a service account's client, and its user with
// it.
func (p *Provider) RemoveServiceAccount(ctx context.Context, req *connect.Request[authv1.RemoveServiceAccountRequest]) (*connect.Response[authv1.RemoveServiceAccountResponse], error) {
	caller, err := adminCaller(ctx)
	if err != nil {
		return nil, err
	}
	if req.Msg.GetUserId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id is required"))
	}
	client, err := p.requireServiceAccount(ctx, caller.TenantExternalID, req.Msg.GetUserId())
	if err != nil {
		return nil, err
	}
	if err := p.admin.deleteClient(ctx, client.ID); err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&authv1.RemoveServiceAccountResponse{}), nil
}
