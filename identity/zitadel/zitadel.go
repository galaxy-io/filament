// Package zitadel adapts a self-hosted Zitadel instance to filament's
// identity port. Filament owns every screen and the AuthService contract;
// Zitadel stores credentials, mints tokens, and federates upstream IdPs.
//
// It is a separate module so its gRPC and SDK dependencies stay out of the
// core module every connector builds against.
package zitadel

import (
	"context"
	"errors"
	"fmt"
	"net/url"
	"strings"
	"sync"

	"connectrpc.com/connect"
	"github.com/coreos/go-oidc/v3/oidc"
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
	// UIOrigin is the browser origin filament's login page is served from;
	// Zitadel redirects sign-in there.
	UIOrigin string
}

// Provider implements filament's identity port against Zitadel.
type Provider struct {
	issuer   string
	clientID string
	api      *zclient.Client
	verifier *oidc.IDTokenVerifier
	project  projectRef
	// rolesClaim is precomputed; every authenticated request reads it.
	rolesClaim string
	// projectGrants caches org id -> project grant id.
	projectGrants sync.Map
}

// New provisions filament's application in Zitadel and returns a provider
// ready to authenticate. Provisioning is idempotent: every boot converges
// the same instance state.
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

	p := &Provider{issuer: issuer, api: api}
	if err := p.bootstrap(ctx, opts.UIOrigin); err != nil {
		_ = api.Close()
		return nil, fmt.Errorf("zitadel bootstrap: %w", err)
	}
	oidcProvider, err := oidc.NewProvider(ctx, issuer)
	if err != nil {
		_ = api.Close()
		return nil, fmt.Errorf("discover issuer %s: %w", issuer, err)
	}
	p.verifier = oidcProvider.Verifier(&oidc.Config{SkipClientIDCheck: true})
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

// Authenticate verifies the access token against the issuer's JWKS and
// resolves the caller's tenant and roles from its claims.
func (p *Provider) Authenticate(ctx context.Context, bearer string) (identity.Caller, error) {
	token, err := p.verifier.Verify(ctx, bearer)
	if err != nil {
		return identity.Caller{}, err
	}
	var claims map[string]any
	if err := token.Claims(&claims); err != nil {
		return identity.Caller{}, err
	}
	orgID, _ := claims[claimOrgID].(string)
	if orgID == "" {
		return identity.Caller{}, errors.New("token carries no organization; request the resourceowner scope")
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
	return caller, nil
}

// GetAuthConfig returns what the UI needs to run the PKCE flow.
func (p *Provider) GetAuthConfig(_ context.Context, _ *connect.Request[authv1.GetAuthConfigRequest]) (*connect.Response[authv1.GetAuthConfigResponse], error) {
	return connect.NewResponse(&authv1.GetAuthConfigResponse{
		Issuer:   p.issuer,
		ClientId: p.clientID,
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
