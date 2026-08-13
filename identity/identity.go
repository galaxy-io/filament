// Package identity is filament's authentication port. It is optional: with
// no provider configured the API is unauthenticated and every request falls
// back to the default tenant, so nothing on the data path imports this
// package.
package identity

import (
	"context"
	"slices"

	"github.com/galaxy-io/filament"
	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
)

// Provider serves AuthService and authenticates the bearer tokens its
// session flow hands out. Implementing the generated handler is the bulk of
// it; Authenticate is the one method the RPC interceptor needs that no
// service definition covers.
type Provider interface {
	authv1connect.AuthServiceHandler

	// Authenticate resolves a bearer token to its caller, failing when the
	// token is invalid or carries no tenant.
	Authenticate(ctx context.Context, bearer string) (Caller, error)
}

// Caller is the authenticated identity behind a request.
type Caller struct {
	// UserID is the provider's stable user identifier.
	UserID string
	// Tenant is the caller's tenant; a provider maps its own organization
	// or workspace concept onto it.
	Tenant filament.TenantID
	// TenantName is a display name for the tenant when the provider has one.
	TenantName string
	Roles      []authv1.Role
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

type callerCtxKey struct{}

// WithCaller returns a context carrying the authenticated caller.
func WithCaller(ctx context.Context, caller Caller) context.Context {
	return context.WithValue(ctx, callerCtxKey{}, caller)
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
	caller, ok := CallerFrom(ctx)
	if !ok || caller.Tenant == "" {
		return "", false
	}
	return caller.Tenant, true
}
