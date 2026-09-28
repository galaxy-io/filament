// Package keycloak adapts a Keycloak realm to filament's identity port.
// Filament owns every screen and the AuthService contract; Keycloak stores
// credentials, issues tokens, and holds the organizations that are
// filament's tenants. The server is Keycloak's only client: browsers hold a
// session cookie and never learn the issuer, and service accounts mint
// client-credentials tokens against the realm.
//
// Human sign-in uses the direct access grant, so filament's own login page
// keeps working and nothing redirects to Keycloak. Anything Keycloak offers
// beyond a password, such as MFA or a federated login, is not reachable
// through it.
//
// Given the Keycloak admin's credentials, filament creates its realm and its
// own confidential client on boot; without them the realm's owners provide
// both, with the client's service account holding realm management rights.
// Everything else filament needs, it converges on boot either way:
// organizations, its roles on the client, the token shape, and the attribute
// policy invitations rely on. Keycloak 26 or newer.
//
// It is a separate module so its OIDC dependencies stay out of the core
// module every connector builds against.
package keycloak

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/clientcredentials"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
	"github.com/galaxy-io/filament/identity"
)

// Options configures a Provider.
type Options struct {
	// Issuer is the realm URL, the issuer its tokens carry.
	Issuer string
	// ClientID and ClientSecret identify filament's confidential client. Its
	// service account administers the realm on filament's behalf, and every
	// token filament accepts names it as an audience.
	ClientID     string
	ClientSecret string
	// UIOrigin is the origin the UI is served from; https marks the session
	// cookie Secure.
	UIOrigin string
	// AdminUsername and AdminPassword, when set, are the Keycloak admin's
	// credentials on the master realm. Filament uses them once per boot to
	// create its realm and client when they are missing. Both or neither.
	AdminUsername string
	AdminPassword string
	// Bootstrap, when set, is a tenant and an admin service account the
	// provider converges on every boot, so a deployment can be driven by an
	// SDK before any person has registered. All three fields or none.
	Bootstrap Bootstrap
}

// Bootstrap names a tenant and the service account that administers it.
type Bootstrap struct {
	Tenant       string
	ClientID     string
	ClientSecret string
}

func (b Bootstrap) set() bool { return b.Tenant != "" || b.ClientID != "" || b.ClientSecret != "" }

func (b Bootstrap) complete() bool { return b.Tenant != "" && b.ClientID != "" && b.ClientSecret != "" }

// Provider implements filament's identity port against Keycloak.
type Provider struct {
	authv1connect.UnimplementedAuthServiceHandler

	issuer        string
	clientID      string
	clientSecret  string
	tokenEndpoint string
	verifier      *oidc.IDTokenVerifier
	admin         *admin
	// client is the uuid Keycloak knows filament's client by.
	client string
	// roles caches the client's filament roles by key; grants need the
	// representation, not the name.
	roles map[string]roleRep
	// secureCookies marks the session cookie Secure when the UI is served
	// over https.
	secureCookies bool
	// callers caches session cookie -> cachedCaller so a browser's requests
	// do not each round-trip to Keycloak.
	callers sync.Map
}

// New discovers the realm, proves the client credentials, converges the
// realm, and returns a provider ready to authenticate.
func New(ctx context.Context, opts Options) (*Provider, error) {
	if opts.Issuer == "" {
		return nil, errors.New("keycloak: issuer is required")
	}
	if opts.ClientID == "" || opts.ClientSecret == "" {
		return nil, errors.New("keycloak: client id and secret are required")
	}
	if opts.Bootstrap.set() && !opts.Bootstrap.complete() {
		return nil, errors.New("keycloak: bootstrap needs a tenant, a client id, and a client secret")
	}
	if (opts.AdminUsername == "") != (opts.AdminPassword == "") {
		return nil, errors.New("keycloak: admin username and password go together")
	}
	issuer := strings.TrimRight(opts.Issuer, "/")
	server, realmName, err := splitIssuer(issuer)
	if err != nil {
		return nil, err
	}
	adminBase := server + "/admin/realms/" + realmName
	// The realm has to exist before discovery can find it.
	if opts.AdminUsername != "" {
		if err := ensureRealm(ctx, server, realmName, opts); err != nil {
			return nil, fmt.Errorf("keycloak: create realm %q: %w", realmName, err)
		}
	}
	realm, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		return nil, fmt.Errorf("keycloak: discover %s: %w", issuer, err)
	}
	credentials := clientcredentials.Config{
		ClientID:     opts.ClientID,
		ClientSecret: strings.TrimSpace(opts.ClientSecret),
		TokenURL:     realm.Endpoint().TokenURL,
		AuthStyle:    oauth2.AuthStyleInHeader,
	}
	// A wrong secret fails the boot, not the first request that needs it.
	if _, err := credentials.Token(ctx); err != nil {
		return nil, fmt.Errorf("keycloak: authenticate client %q: %w", opts.ClientID, err)
	}
	p := &Provider{
		issuer:        issuer,
		clientID:      opts.ClientID,
		clientSecret:  credentials.ClientSecret,
		tokenEndpoint: realm.Endpoint().TokenURL,
		verifier:      realm.Verifier(&oidc.Config{ClientID: opts.ClientID}),
		// The token source outlives boot and refreshes on this context.
		admin:         &admin{base: adminBase, client: credentials.Client(context.WithoutCancel(ctx))},
		secureCookies: strings.HasPrefix(opts.UIOrigin, "https://"),
	}
	if err := p.bootstrap(ctx); err != nil {
		return nil, fmt.Errorf("keycloak bootstrap: %w", err)
	}
	if opts.Bootstrap.complete() {
		if err := p.bootstrapTenant(ctx, opts.Bootstrap); err != nil {
			return nil, fmt.Errorf("keycloak bootstrap tenant %q: %w", opts.Bootstrap.Tenant, err)
		}
	}
	return p, nil
}

// splitIssuer takes the realm's issuer apart into the server and the realm
// name, since Keycloak forms it as <server>/realms/<realm>.
func splitIssuer(issuer string) (server, realm string, err error) {
	u, err := url.Parse(issuer)
	if err != nil {
		return "", "", fmt.Errorf("keycloak: parse issuer %q: %w", issuer, err)
	}
	prefix, realm, ok := strings.Cut(u.Path, "/realms/")
	if !ok || realm == "" || strings.Contains(realm, "/") {
		return "", "", fmt.Errorf("keycloak: issuer %q is not a realm URL (<server>/realms/<realm>)", issuer)
	}
	u.Path = prefix
	return strings.TrimRight(u.String(), "/"), realm, nil
}

// Authenticate resolves the caller behind a request. Service accounts carry
// a bearer access token verified against the realm's JWKS; browsers carry
// the session cookie Login set.
func (p *Provider) Authenticate(ctx context.Context, header http.Header) (identity.Caller, error) {
	if bearer, ok := strings.CutPrefix(header.Get("Authorization"), "Bearer "); ok && bearer != "" {
		return p.authenticateToken(ctx, bearer)
	}
	if session, ok := sessionFromCookie(header); ok {
		return p.authenticateSession(ctx, session)
	}
	return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("no credentials"))
}

// claims are the parts of a Keycloak access token filament reads. The
// organization scope lists the caller's memberships by alias; the client
// roles mapper puts granted roles under the client.
type claims struct {
	Subject        string   `json:"sub"`
	Organizations  []string `json:"organization"`
	ResourceAccess map[string]struct {
		Roles []string `json:"roles"`
	} `json:"resource_access"`
}

// authenticateToken verifies an access token and resolves the caller's
// tenant and roles from its claims. The organization alias is the tenant's
// external id: Keycloak fixes it at creation and puts nothing else on the
// token.
func (p *Provider) authenticateToken(ctx context.Context, bearer string) (identity.Caller, error) {
	token, err := p.verifier.Verify(ctx, bearer)
	if err != nil {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, err)
	}
	var c claims
	if err := token.Claims(&c); err != nil {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if len(c.Organizations) == 0 {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("token carries no organization; request the organization scope"))
	}
	if len(c.Organizations) > 1 {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("token carries more than one organization; request one with the organization:<alias> scope"))
	}
	caller := identity.Caller{
		UserID:           c.Subject,
		TenantExternalID: c.Organizations[0],
		TenantName:       c.Organizations[0],
	}
	for _, key := range c.ResourceAccess[p.clientID].Roles {
		if role := identity.RoleFromKey(key); role != authv1.Role_ROLE_UNSPECIFIED {
			caller.Roles = append(caller.Roles, role)
		}
	}
	if len(caller.Roles) == 0 {
		return identity.Caller{}, connect.NewError(connect.CodePermissionDenied, errors.New("token carries no Filament role"))
	}
	return caller, nil
}

// GetAuthConfig tells the UI that auth is on and non-interactive clients
// what to request from the issuer.
func (p *Provider) GetAuthConfig(_ context.Context, _ *connect.Request[authv1.GetAuthConfigRequest]) (*connect.Response[authv1.GetAuthConfigResponse], error) {
	return connect.NewResponse(&authv1.GetAuthConfigResponse{
		Issuer:               p.issuer,
		ServiceAccountScopes: []string{"openid", "organization"},
	}), nil
}

// adminCaller resolves the caller the interceptor stashed and requires
// tenant administration.
func adminCaller(ctx context.Context) (identity.Caller, error) {
	c, ok := identity.CallerFrom(ctx)
	if !ok {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	if !c.IsAdmin() {
		return identity.Caller{}, connect.NewError(connect.CodePermissionDenied, errors.New("only admins can manage the team"))
	}
	return c, nil
}

// tenant resolves the caller's organization, which admin requests are
// scoped to.
func (p *Provider) tenant(ctx context.Context, caller identity.Caller) (organizationRep, error) {
	org, err := p.admin.organizationByAlias(ctx, caller.TenantExternalID)
	if err != nil {
		return organizationRep{}, rpcError(err)
	}
	return org, nil
}

// rpcError maps a Keycloak failure onto the code a client should see. An
// outage keeps its own code so it is never mistaken for a rejection.
func rpcError(err error) error {
	var cerr *connect.Error
	if errors.As(err, &cerr) {
		return err
	}
	if errors.Is(err, errUnavailable) {
		return connect.NewError(connect.CodeUnavailable, err)
	}
	switch {
	case isStatus(err, http.StatusNotFound):
		return connect.NewError(connect.CodeNotFound, err)
	case isStatus(err, http.StatusConflict), isStatus(err, http.StatusBadRequest):
		return connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewError(connect.CodeInternal, err)
}
