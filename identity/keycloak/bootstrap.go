package keycloak

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"golang.org/x/oauth2"

	"github.com/galaxy-io/filament"
	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

// realmManagementRoles are the rights filament's service account needs on
// its realm: users, clients, and the realm's own settings.
var realmManagementRoles = []string{"manage-realm", "manage-users", "manage-clients", "view-realm", "view-users", "view-clients"}

// ensureRealm creates filament's realm and client as the Keycloak admin when
// they are missing, and converges the client's secret on the configured one.
// The realm requires TLS only when the issuer itself is served over it.
func ensureRealm(ctx context.Context, server, realm string, opts Options) error {
	token, err := adminToken(ctx, server, opts.AdminUsername, opts.AdminPassword)
	if err != nil {
		return err
	}
	client := oauth2.NewClient(ctx, oauth2.StaticTokenSource(&oauth2.Token{AccessToken: token}))
	realms := &admin{base: server + "/admin/realms", client: client}
	if _, err := realms.do(ctx, http.MethodGet, "/"+url.PathEscape(realm), nil, nil); isStatus(err, http.StatusNotFound) {
		sslRequired := "external"
		if strings.HasPrefix(server, "http://") {
			sslRequired = "none"
		}
		if _, err := realms.do(ctx, http.MethodPost, "", map[string]any{
			"realm":                realm,
			"enabled":              true,
			"sslRequired":          sslRequired,
			"organizationsEnabled": true,
		}, nil); err != nil {
			return err
		}
	} else if err != nil {
		return err
	}
	a := &admin{base: server + "/admin/realms/" + realm, client: client}
	found, err := a.clientByClientID(ctx, opts.ClientID)
	if isStatus(err, http.StatusNotFound) {
		id, err := a.createClient(ctx, clientRep{
			ClientID:                  opts.ClientID,
			Secret:                    strings.TrimSpace(opts.ClientSecret),
			Name:                      "Filament",
			ServiceAccountsEnabled:    true,
			DirectAccessGrantsEnabled: true,
		})
		if err != nil {
			return fmt.Errorf("create client: %w", err)
		}
		found = clientRep{ID: id, ClientID: opts.ClientID}
	} else if err != nil {
		return fmt.Errorf("resolve client: %w", err)
	}
	current, err := a.clientSecret(ctx, found.ID)
	if err != nil {
		return fmt.Errorf("read client secret: %w", err)
	}
	if current != strings.TrimSpace(opts.ClientSecret) {
		raw, err := a.getClientRaw(ctx, found.ID)
		if err != nil {
			return fmt.Errorf("read client: %w", err)
		}
		raw["secret"] = strings.TrimSpace(opts.ClientSecret)
		if err := a.putClient(ctx, found.ID, raw); err != nil {
			return fmt.Errorf("set client secret: %w", err)
		}
	}
	user, err := a.serviceAccountUser(ctx, found.ID)
	if err != nil {
		return fmt.Errorf("resolve service account user: %w", err)
	}
	management, err := a.clientByClientID(ctx, "realm-management")
	if err != nil {
		return fmt.Errorf("resolve realm-management client: %w", err)
	}
	roles, err := a.clientRoles(ctx, management.ID)
	if err != nil {
		return fmt.Errorf("list realm-management roles: %w", err)
	}
	var grant []roleRep
	for _, role := range roles {
		if slices.Contains(realmManagementRoles, role.Name) {
			grant = append(grant, role)
		}
	}
	if err := a.addUserClientRoles(ctx, user.ID, management.ID, grant); err != nil {
		return fmt.Errorf("grant realm management: %w", err)
	}
	return nil
}

// adminToken signs the Keycloak admin in on the master realm.
func adminToken(ctx context.Context, server, username, password string) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, tokenTimeout)
	defer cancel()
	form := url.Values{
		"grant_type": {"password"},
		"client_id":  {"admin-cli"},
		"username":   {username},
		"password":   {password},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, server+"/realms/master/protocol/openid-connect/token", strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("%w: %w", errUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()
	var token struct {
		AccessToken string `json:"access_token"`
	}
	if res.StatusCode != http.StatusOK {
		return "", fmt.Errorf("admin sign-in on master realm: status %d", res.StatusCode)
	}
	if err := decodeJSON(res.Body, &token); err != nil || token.AccessToken == "" {
		return "", fmt.Errorf("admin sign-in on master realm: no token")
	}
	return token.AccessToken, nil
}

// bootstrap converges the realm on the configuration filament needs and
// caches what later requests refer to. Every step is idempotent, so each
// boot lands on the same state. The client itself must already exist: it is
// the credential filament boots with.
func (p *Provider) bootstrap(ctx context.Context) error {
	client, err := p.admin.clientByClientID(ctx, p.clientID)
	if err != nil {
		return fmt.Errorf("resolve client: %w", err)
	}
	p.client = client.ID
	if err := p.ensureOrganizations(ctx); err != nil {
		return err
	}
	if err := p.ensureRoles(ctx); err != nil {
		return err
	}
	if err := p.ensureTokenShape(ctx, p.client); err != nil {
		return err
	}
	return p.ensureUnmanagedAttributes(ctx)
}

// ensureOrganizations turns the realm's organizations on; they are the
// tenants.
func (p *Provider) ensureOrganizations(ctx context.Context) error {
	realm, err := p.admin.realm(ctx)
	if err != nil {
		return fmt.Errorf("read realm: %w", err)
	}
	if enabled, _ := realm["organizationsEnabled"].(bool); enabled {
		return nil
	}
	if err := p.admin.updateRealm(ctx, map[string]any{"organizationsEnabled": true}); err != nil {
		return fmt.Errorf("enable organizations: %w", err)
	}
	return nil
}

// ensureRoles adds any filament role the client is missing and caches
// their representations, which grants need.
func (p *Provider) ensureRoles(ctx context.Context) error {
	existing, err := p.admin.clientRoles(ctx, p.client)
	if err != nil {
		return fmt.Errorf("list client roles: %w", err)
	}
	have := make(map[string]roleRep, len(existing))
	for _, role := range existing {
		have[role.Name] = role
	}
	for _, key := range identity.RoleKeys() {
		if _, ok := have[key]; ok {
			continue
		}
		if err := p.admin.createClientRole(ctx, p.client, key); err != nil {
			return fmt.Errorf("add client role %s: %w", key, err)
		}
	}
	created, err := p.admin.clientRoles(ctx, p.client)
	if err != nil {
		return fmt.Errorf("list client roles: %w", err)
	}
	p.roles = make(map[string]roleRep, len(created))
	for _, role := range created {
		p.roles[role.Name] = role
	}
	return nil
}

// ensureTokenShape makes a client's tokens verifiable by filament: they name
// filament as an audience. Filament's own client and every service account
// go through it. The organization membership is not part of the shape:
// Keycloak keeps that scope optional whatever a client's defaults say, so
// every grant filament makes or advertises requests it explicitly.
func (p *Provider) ensureTokenShape(ctx context.Context, clientID string) error {
	mappers, err := p.admin.protocolMappers(ctx, clientID)
	if err != nil {
		return fmt.Errorf("list protocol mappers: %w", err)
	}
	want := audienceMapper(p.clientID)
	hasAudience := false
	for _, mapper := range mappers {
		if mapper.ProtocolMapper == want.ProtocolMapper && mapper.Config["included.client.audience"] == p.clientID {
			hasAudience = true
			break
		}
	}
	if !hasAudience {
		if err := p.admin.addProtocolMapper(ctx, clientID, want); err != nil {
			return fmt.Errorf("add audience mapper: %w", err)
		}
	}
	return nil
}

// ensureUnmanagedAttributes lets filament keep its own attributes on users,
// which invitations rely on. Keycloak drops attributes its user profile
// does not declare unless the realm allows administrators to set them.
func (p *Provider) ensureUnmanagedAttributes(ctx context.Context) error {
	profile, err := p.admin.userProfile(ctx)
	if err != nil {
		return fmt.Errorf("read user profile: %w", err)
	}
	if policy, _ := profile["unmanagedAttributePolicy"].(string); policy == "ADMIN_EDIT" || policy == "ENABLED" {
		return nil
	}
	profile["unmanagedAttributePolicy"] = "ADMIN_EDIT"
	if err := p.admin.updateUserProfile(ctx, profile); err != nil {
		return fmt.Errorf("allow unmanaged attributes: %w", err)
	}
	return nil
}

// grantRole gives a user one filament role, replacing any it held.
func (p *Provider) grantRole(ctx context.Context, userID, roleKey string) error {
	held, err := p.admin.userClientRoles(ctx, userID, p.client)
	if err != nil {
		return err
	}
	if err := p.admin.removeUserClientRoles(ctx, userID, p.client, held); err != nil {
		return err
	}
	role, ok := p.roles[roleKey]
	if !ok {
		return fmt.Errorf("unknown role %q", roleKey)
	}
	return p.admin.addUserClientRoles(ctx, userID, p.client, []roleRep{role})
}

// bootstrapTenant converges the tenant and whoever gets in first. Nothing
// changes when it is all already there.
func (p *Provider) bootstrapTenant(ctx context.Context, b Bootstrap) error {
	tenant := alias(b.Tenant)
	org, err := p.admin.organizationByAlias(ctx, tenant)
	if isStatus(err, http.StatusNotFound) {
		id, err := p.admin.createOrganization(ctx, organizationRep{Name: b.Tenant, Alias: tenant})
		if err != nil {
			return fmt.Errorf("create organization: %w", err)
		}
		org = organizationRep{ID: id, Alias: tenant}
	} else if err != nil {
		return fmt.Errorf("resolve organization: %w", err)
	}
	if b.ClientID != "" {
		if err := p.bootstrapServiceAccount(ctx, org, b); err != nil {
			return err
		}
	}
	if b.AdminEmail != "" {
		return p.bootstrapAdmin(ctx, org, b.AdminEmail)
	}
	return nil
}

// bootstrapServiceAccount converges the admin service account that drives
// the tenant, with the secret the deployer chose. An SDK can then act on the
// deployment before any person has signed in. A secret rotated elsewhere is
// set back on the next boot, since the configured value is the one the
// deployer holds.
func (p *Provider) bootstrapServiceAccount(ctx context.Context, org organizationRep, b Bootstrap) error {
	attributes := map[string]string{managedKey: "true", tenantKey: org.Alias}
	client, err := p.admin.clientByClientID(ctx, b.ClientID)
	if isStatus(err, http.StatusNotFound) {
		id, err := p.admin.createClient(ctx, clientRep{
			ClientID:               b.ClientID,
			Secret:                 b.ClientSecret,
			Name:                   b.ClientID,
			ServiceAccountsEnabled: true,
			Attributes:             attributes,
		})
		if err != nil {
			return fmt.Errorf("create client: %w", err)
		}
		client = clientRep{ID: id, ClientID: b.ClientID, Attributes: attributes}
	} else if err != nil {
		return fmt.Errorf("resolve client: %w", err)
	}
	// A client filament did not create, or one that belongs to another
	// tenant, is never taken over.
	if client.Attributes[managedKey] != "true" || client.Attributes[tenantKey] != org.Alias {
		return fmt.Errorf("client %q exists and is not this tenant's service account", b.ClientID)
	}
	current, err := p.admin.clientSecret(ctx, client.ID)
	if err != nil {
		return fmt.Errorf("read client secret: %w", err)
	}
	if current != b.ClientSecret {
		raw, err := p.admin.getClientRaw(ctx, client.ID)
		if err != nil {
			return fmt.Errorf("read client: %w", err)
		}
		raw["secret"] = b.ClientSecret
		if err := p.admin.putClient(ctx, client.ID, raw); err != nil {
			return fmt.Errorf("set client secret: %w", err)
		}
	}
	if err := p.ensureTokenShape(ctx, client.ID); err != nil {
		return err
	}
	user, err := p.admin.serviceAccountUser(ctx, client.ID)
	if err != nil {
		return fmt.Errorf("resolve service account user: %w", err)
	}
	if err := p.admin.addMember(ctx, org.ID, user.ID); err != nil && !isStatus(err, http.StatusConflict) {
		return fmt.Errorf("add member: %w", err)
	}
	if err := p.grantRole(ctx, user.ID, identity.RoleKey(authv1.Role_ROLE_ADMIN)); err != nil {
		return fmt.Errorf("grant admin: %w", err)
	}
	return nil
}

// bootstrapAdmin converges the person invited to administer the tenant. A
// new user is invited with the Admin role and the link is logged. An
// unredeemed invitation is reissued, so a lost link is one restart away. A
// redeemed one is left alone: a restart never logs a way into an account
// someone already holds, and the members page owns the role from then on.
//
// The realm's user profile requires a name before anyone can sign in; the
// address supplies one until the admin is known by a better one.
func (p *Provider) bootstrapAdmin(ctx context.Context, org organizationRep, email string) error {
	user, err := p.admin.userByEmail(ctx, email)
	if isStatus(err, http.StatusNotFound) {
		code, attributes, err := newInvite()
		if err != nil {
			return err
		}
		local, _, _ := strings.Cut(email, "@")
		id, err := p.admin.createUser(ctx, userRep{
			Username:   email,
			Email:      email,
			FirstName:  local,
			LastName:   local,
			Enabled:    true,
			Attributes: attributes,
		})
		if err != nil {
			return fmt.Errorf("create admin: %w", err)
		}
		if err := p.admin.addMember(ctx, org.ID, id); err != nil {
			return fmt.Errorf("add member: %w", p.discardUser(ctx, id, err))
		}
		if err := p.grantRole(ctx, id, identity.RoleKey(authv1.Role_ROLE_ADMIN)); err != nil {
			return fmt.Errorf("grant admin: %w", p.discardUser(ctx, id, err))
		}
		p.logInvite(email, id, code)
		return nil
	} else if err != nil {
		return fmt.Errorf("resolve admin: %w", err)
	}
	raw, err := p.admin.getUserRaw(ctx, user.ID)
	if err != nil {
		return fmt.Errorf("read admin: %w", err)
	}
	attributes, _ := raw["attributes"].(map[string]any)
	if attribute(attributes, inviteHashKey) == "" {
		return nil
	}
	code, fresh, err := newInvite()
	if err != nil {
		return err
	}
	for key, values := range fresh {
		attributes[key] = values
	}
	raw["attributes"] = attributes
	if err := p.admin.putUser(ctx, user.ID, raw); err != nil {
		return fmt.Errorf("reissue invitation: %w", err)
	}
	p.logInvite(email, user.ID, code)
	return nil
}

// logInvite hands the deployer the link that redeems the admin's invitation.
func (p *Provider) logInvite(email, userID, code string) {
	p.log.Info("bootstrap admin invited; open the link to set a password",
		filament.Field{Key: "email", Value: email},
		filament.Field{Key: "url", Value: identity.InviteURL(p.uiOrigin, userID, code)},
	)
}
