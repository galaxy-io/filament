// Package zitadel adapts a self-hosted Zitadel instance to filament's
// identity port. Filament owns every screen and the AuthService contract;
// Zitadel stores credentials, holds sessions, and mints the tokens service
// accounts use. The server is Zitadel's only client: browsers hold a session
// cookie and never learn the issuer.
//
// It is a separate module so its gRPC and SDK dependencies stay out of the
// core module every connector builds against.
package zitadel

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"github.com/zitadel/zitadel-go/v3/pkg/authorization"
	zitadeloauth "github.com/zitadel/zitadel-go/v3/pkg/authorization/oauth"
	zclient "github.com/zitadel/zitadel-go/v3/pkg/client"
	"github.com/zitadel/zitadel-go/v3/pkg/zitadel"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

// Zitadel puts the user's organization on the token when the client requests
// the resourceowner scope. The organization is the tenant.
const (
	claimOrgID   = "urn:zitadel:iam:user:resourceowner:id"
	claimOrgName = "urn:zitadel:iam:user:resourceowner:name"
)

// Options configures a Provider.
type Options struct {
	// Issuer is the Zitadel base URL.
	Issuer string
	// PAT is a machine-user personal access token holding instance rights.
	PAT string
	// UIOrigin is the origin the UI is served from; https marks the session
	// cookie Secure.
	UIOrigin string
}

// Provider implements filament's identity port against Zitadel.
type Provider struct {
	issuer   string
	api      *zclient.Client
	verifier authorization.Verifier[*zitadeloauth.IntrospectionContext]
	project  projectRef
	// rolesClaim is precomputed; every token-authenticated request reads it.
	rolesClaim string
	// secureCookies marks the session cookie Secure when the UI is served
	// over https.
	secureCookies bool
	// callers caches sessionRef -> cachedCaller so a browser's requests do
	// not each round-trip to Zitadel.
	callers sync.Map
	// projectGrants caches org id -> project grant id.
	projectGrants sync.Map
}

// New provisions filament's project in Zitadel and returns a provider ready
// to authenticate. Provisioning is idempotent: every boot converges the same
// instance state.
func New(ctx context.Context, opts Options) (*Provider, error) {
	if opts.Issuer == "" {
		return nil, errors.New("zitadel: issuer is required")
	}
	if opts.PAT == "" {
		return nil, errors.New("zitadel: personal access token is required")
	}
	issuer := strings.TrimRight(opts.Issuer, "/")
	target, err := dialTarget(issuer)
	if err != nil {
		return nil, err
	}
	api, err := zclient.New(ctx, target, zclient.WithAuth(zclient.PAT(strings.TrimSpace(opts.PAT))))
	if err != nil {
		return nil, fmt.Errorf("zitadel: connect %s: %w", issuer, err)
	}

	p := &Provider{issuer: issuer, api: api, secureCookies: strings.HasPrefix(opts.UIOrigin, "https://")}
	if err := p.bootstrap(ctx); err != nil {
		_ = api.Close()
		return nil, fmt.Errorf("zitadel bootstrap: %w", err)
	}
	verifier, err := zitadeloauth.WithJWT(p.project.id, http.DefaultClient)(ctx, target)
	if err != nil {
		_ = api.Close()
		return nil, fmt.Errorf("initialize access-token verifier for %s: %w", issuer, err)
	}
	p.verifier = verifier
	p.rolesClaim = "urn:zitadel:iam:org:project:" + p.project.id + ":roles"
	return p, nil
}

// dialTarget turns the issuer URL into the SDK's host description, keeping
// plaintext local instances working.
func dialTarget(issuer string) (*zitadel.Zitadel, error) {
	u, err := url.Parse(issuer)
	if err != nil {
		return nil, fmt.Errorf("zitadel: parse issuer %q: %w", issuer, err)
	}
	if u.Scheme == "https" {
		return zitadel.New(u.Hostname()), nil
	}
	port := u.Port()
	if port == "" {
		port = "80"
	}
	return zitadel.New(u.Hostname(), zitadel.WithInsecure(port)), nil
}

// Close releases the provider's connection to Zitadel.
func (p *Provider) Close() error { return p.api.Close() }

// Authenticate resolves the caller behind a request. Service accounts carry
// a bearer access token verified against the issuer's JWKS; browsers carry
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

// authenticateToken verifies an access token and resolves the caller's
// tenant and roles from its claims.
func (p *Provider) authenticateToken(ctx context.Context, bearer string) (identity.Caller, error) {
	token, err := p.verifier.CheckAuthorization(ctx, "Bearer "+bearer)
	if err != nil {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, err)
	}
	claims := token.Claims
	orgID, _ := claims[claimOrgID].(string)
	if orgID == "" {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("token carries no organization; request the resourceowner scope"))
	}
	orgName, _ := claims[claimOrgName].(string)
	userID, _ := claims["sub"].(string)

	caller := identity.Caller{
		UserID:           userID,
		TenantExternalID: orgID,
		TenantName:       orgName,
	}
	if granted, ok := claims[p.rolesClaim].(map[string]any); ok {
		caller.Roles = make([]authv1.Role, 0, len(granted))
		for key := range granted {
			if role := identity.RoleFromKey(key); role != authv1.Role_ROLE_UNSPECIFIED {
				caller.Roles = append(caller.Roles, role)
			}
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
		Issuer: p.issuer,
		ServiceAccountScopes: []string{
			"openid",
			"urn:zitadel:iam:user:resourceowner",
			"urn:zitadel:iam:org:project:id:" + p.project.id + ":aud",
			"urn:zitadel:iam:org:projects:roles",
		},
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
