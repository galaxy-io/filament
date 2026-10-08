// Package proxy trusts identity headers set by an authenticating gateway in
// front of filament. It verifies nothing itself: the gateway has already
// authenticated the person, and the deployment's network policy is what
// makes the headers trustworthy. Sign-in, members and service accounts live
// upstream, so this is an Authenticator and not a Provider.
package proxy

import (
	"context"
	"errors"
	"net/http"

	"connectrpc.com/connect"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

// Options name the headers to trust and the one tenant to serve.
type Options struct {
	// UserHeader carries the gateway's stable user id.
	UserHeader string
	// TenantHeader carries the gateway's tenant id.
	TenantHeader string
	// RoleHeader carries one of filament's role keys: admin, creator or
	// viewer. The gateway translates its own vocabulary before setting it.
	// Optional; unset leaves callers without roles.
	RoleHeader string
	// ServiceHeader names a trusted service calling on its own behalf. A
	// request carrying it needs no user header. Optional.
	ServiceHeader string
	// Tenant is the gateway's id for the one tenant this deployment serves.
	// A request for any other tenant is refused.
	Tenant string
	// LoginURL is where people sign in, reported in the auth config document.
	LoginURL string
}

// Authenticator reads callers off trusted gateway headers.
type Authenticator struct {
	opts Options
}

// New returns an Authenticator for opts. The user header, tenant header and
// tenant are required.
func New(opts Options) (*Authenticator, error) {
	if opts.UserHeader == "" || opts.TenantHeader == "" || opts.Tenant == "" {
		return nil, errors.New("proxy: user header, tenant header and tenant are required")
	}
	return &Authenticator{opts: opts}, nil
}

// Authenticate resolves the caller from the configured headers. A missing
// tenant or user is Unauthenticated; a tenant this deployment does not serve
// is PermissionDenied.
func (a *Authenticator) Authenticate(_ context.Context, header http.Header) (identity.Caller, error) {
	tenant := header.Get(a.opts.TenantHeader)
	if tenant == "" {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("no tenant header"))
	}
	if tenant != a.opts.Tenant {
		return identity.Caller{}, connect.NewError(connect.CodePermissionDenied, errors.New("tenant is not served here"))
	}
	if a.opts.ServiceHeader != "" {
		if service := header.Get(a.opts.ServiceHeader); service != "" {
			return identity.Caller{TenantExternalID: tenant, Service: service}, nil
		}
	}
	user := header.Get(a.opts.UserHeader)
	if user == "" {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("no user header"))
	}
	caller := identity.Caller{UserID: user, TenantExternalID: tenant}
	if a.opts.RoleHeader != "" {
		if role := identity.RoleFromKey(header.Get(a.opts.RoleHeader)); role != authv1.Role_ROLE_UNSPECIFIED {
			caller.Roles = []authv1.Role{role}
		}
	}
	return caller, nil
}

// LoginURL reports where people sign in.
func (a *Authenticator) LoginURL() string { return a.opts.LoginURL }

var (
	_ identity.Authenticator = (*Authenticator)(nil)
	_ identity.ExternalLogin = (*Authenticator)(nil)
)
