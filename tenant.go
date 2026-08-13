package filament

import "context"

type tenantCtxKey struct{}

// WithTenant returns a context carrying the authenticated tenant.
func WithTenant(ctx context.Context, tenant TenantID) context.Context {
	return context.WithValue(ctx, tenantCtxKey{}, tenant)
}

// TenantFrom returns the authenticated tenant, if one was established.
func TenantFrom(ctx context.Context) (TenantID, bool) {
	tenant, ok := ctx.Value(tenantCtxKey{}).(TenantID)
	return tenant, ok
}

type rolesCtxKey struct{}

// WithRoles returns a context carrying the authenticated caller's roles.
func WithRoles(ctx context.Context, roles []string) context.Context {
	return context.WithValue(ctx, rolesCtxKey{}, roles)
}

// RolesFrom returns the authenticated caller's roles, if any.
func RolesFrom(ctx context.Context) ([]string, bool) {
	roles, ok := ctx.Value(rolesCtxKey{}).([]string)
	return roles, ok
}
