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

	"github.com/galaxy-io/filament/identity"
)

// Headers the gateway sets. The gateway strips any copy a client sent.
const (
	// UserHeader carries the gateway's stable user id.
	UserHeader = "X-Forwarded-User"
	// TenantHeader carries the gateway's tenant id.
	TenantHeader = "X-Forwarded-Tenant"
	// ServiceHeader names a trusted service calling on its own behalf. A
	// request carrying it needs no user header.
	ServiceHeader = "X-Forwarded-Service"
)

// Authenticator reads callers off trusted gateway headers.
type Authenticator struct {
	tenant string
}

// New returns an Authenticator. A non-empty tenant pins the deployment to
// that gateway tenant and refuses any other; empty serves whatever tenant
// the gateway names.
func New(tenant string) *Authenticator {
	return &Authenticator{tenant: tenant}
}

// Authenticate resolves the caller from the gateway headers. A missing
// tenant or user is Unauthenticated; a tenant other than the pinned one is
// PermissionDenied.
func (a *Authenticator) Authenticate(_ context.Context, header http.Header) (identity.Caller, error) {
	tenant := header.Get(TenantHeader)
	if tenant == "" {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("no tenant header"))
	}
	if a.tenant != "" && tenant != a.tenant {
		return identity.Caller{}, connect.NewError(connect.CodePermissionDenied, errors.New("tenant is not served here"))
	}
	if service := header.Get(ServiceHeader); service != "" {
		return identity.Caller{TenantExternalID: tenant, Service: service}, nil
	}
	user := header.Get(UserHeader)
	if user == "" {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("no user header"))
	}
	return identity.Caller{UserID: user, TenantExternalID: tenant}, nil
}

var _ identity.Authenticator = (*Authenticator)(nil)
