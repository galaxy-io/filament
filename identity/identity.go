// Package identity is filament's authentication port. It is optional: with
// no provider configured the API is unauthenticated and every request falls
// back to the default tenant, so nothing on the data path imports this
// package.
package identity

import (
	"context"
	"net/http"
	"slices"

	"github.com/galaxy-io/filament"
	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
)

// Authenticator resolves the credentials on a request to its caller. It is
// the one method the RPC interceptor needs that no service definition
// covers.
type Authenticator interface {
	// Authenticate resolves the credentials on a request's headers, a bearer
	// token, a session cookie or identity headers set by a trusted gateway,
	// to their caller. It fails when there are none, they are invalid, or
	// they carry no tenant; a *connect.Error keeps its code on the way to
	// the client.
	Authenticate(ctx context.Context, header http.Header) (Caller, error)
}

// Provider is an Authenticator that also serves AuthService: login, session,
// members and service accounts. Implementing the generated handler is the
// bulk of it. An Authenticator that is not a Provider, such as a gateway
// proxy, leaves sign-in to something upstream.
type Provider interface {
	Authenticator
	authv1connect.AuthServiceHandler
}

// ExternalLogin is implemented by an Authenticator whose sign-in happens
// upstream of filament. The server reports the URL in the auth config
// document so clients send people there instead of to filament's own login.
type ExternalLogin interface {
	LoginURL() string
}

// Caller is the authenticated identity behind a request.
type Caller struct {
	// UserID is the provider's stable user identifier.
	UserID string
	// ID is filament's own id for the user, minted from UserID within Tenant
	// on first sight. Providers leave it empty; the server fills it in.
	ID filament.UserID
	// Tenant is filament's own id for the caller's tenant, resolved from
	// TenantExternalID on first sight. Providers leave it empty; the server
	// fills it in.
	Tenant filament.TenantID
	// TenantExternalID is the provider's id for the tenant; a provider maps
	// its own organization or workspace concept onto it.
	TenantExternalID string
	// TenantName is a display name for the tenant when the provider has one.
	TenantName string
	Roles      []authv1.Role
	// Service names a trusted service calling on its own behalf rather than
	// a person. A service caller has a tenant but no user row.
	Service string
}

// HasRole reports whether the caller holds role.
func (c Caller) HasRole(role authv1.Role) bool { return slices.Contains(c.Roles, role) }

// IsAdmin reports whether the caller administers their tenant.
func (c Caller) IsAdmin() bool { return c.HasRole(authv1.Role_ROLE_ADMIN) }

// roleKeys is the wire vocabulary providers store grants under. Keeping the
// mapping here means an adapter never invents its own role names.
var roleKeys = map[authv1.Role]string{
	authv1.Role_ROLE_ADMIN:   "admin",
	authv1.Role_ROLE_CREATOR: "creator",
	authv1.Role_ROLE_VIEWER:  "viewer",
}

// RoleKey returns the provider-facing key for role, empty when unspecified.
func RoleKey(role authv1.Role) string { return roleKeys[role] }

// RoleKeys returns every role key filament recognizes.
func RoleKeys() []string {
	keys := make([]string, 0, len(roleKeys))
	for _, role := range []authv1.Role{
		authv1.Role_ROLE_ADMIN,
		authv1.Role_ROLE_CREATOR,
		authv1.Role_ROLE_VIEWER,
	} {
		keys = append(keys, roleKeys[role])
	}
	return keys
}

// RoleFromKey maps a provider's stored grant back onto filament's
// vocabulary; unknown keys resolve to ROLE_UNSPECIFIED.
func RoleFromKey(key string) authv1.Role {
	for role, roleKey := range roleKeys {
		if roleKey == key {
			return role
		}
	}
	return authv1.Role_ROLE_UNSPECIFIED
}

type (
	callerCtxKey struct{}
	tenantCtxKey struct{}
)

// WithCaller returns a context carrying the authenticated caller.
func WithCaller(ctx context.Context, caller Caller) context.Context {
	return context.WithValue(ctx, callerCtxKey{}, caller)
}

// WithTenant establishes a request's tenant without fabricating an
// authenticated caller. Auth-disabled single-tenant servers use it to place
// their configured tenant on every request context.
func WithTenant(ctx context.Context, tenant filament.TenantID) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, tenant)
}

// CallerFrom returns the authenticated caller, if one was established. It
// reports false whenever auth is disabled, which is how handlers know to
// fall back to the default tenant.
func CallerFrom(ctx context.Context) (Caller, bool) {
	caller, ok := ctx.Value(callerCtxKey{}).(Caller)
	return caller, ok
}

// TenantFrom returns the authenticated caller's tenant, if any.
func TenantFrom(ctx context.Context) (filament.TenantID, bool) {
	if tenant, ok := ctx.Value(tenantCtxKey{}).(filament.TenantID); ok && tenant != "" {
		return tenant, true
	}
	caller, ok := CallerFrom(ctx)
	if !ok || caller.Tenant == "" {
		return "", false
	}
	return caller.Tenant, true
}
