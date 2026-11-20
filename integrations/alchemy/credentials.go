package alchemy

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/identity/proxy"
)

// Credentials authenticates an outbound call as one tenant, named by its identity provider id.
type Credentials interface {
	Apply(ctx context.Context, tenant string, h http.Header) error
}

// ProxyCredentials asserts the tenant to an alchemy behind the same trusted gateway.
type ProxyCredentials struct {
	Service string
}

// Apply names tenant and the calling service in the gateway's trusted headers.
func (c ProxyCredentials) Apply(_ context.Context, tenant string, h http.Header) error {
	h.Set(proxy.TenantHeader, tenant)
	h.Set(proxy.ServiceHeader, c.Service)
	return nil
}

type tenantKey struct{}

// WithTenant scopes alchemy calls made with ctx to tenant.
func WithTenant(ctx context.Context, tenant filament.TenantID) context.Context {
	return context.WithValue(ctx, tenantKey{}, tenant)
}

// authenticator applies credentials for the context's tenant, caching provider ids since they never change.
type authenticator struct {
	creds   Credentials
	tenants filament.TenantDirectory
	ids     sync.Map
}

func (a *authenticator) intercept(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		tenant, ok := ctx.Value(tenantKey{}).(filament.TenantID)
		if !ok || tenant == "" {
			return nil, connect.NewError(connect.CodeInternal, errors.New("alchemy: call has no tenant"))
		}
		ext, err := a.externalID(ctx, tenant)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		if err := a.creds.Apply(ctx, ext, req.Header()); err != nil {
			return nil, connect.NewError(connect.CodeUnauthenticated, fmt.Errorf("alchemy credentials: %w", err))
		}
		return next(ctx, req)
	}
}

func (a *authenticator) externalID(ctx context.Context, tenant filament.TenantID) (string, error) {
	if ext, ok := a.ids.Load(tenant); ok {
		return ext.(string), nil
	}
	ext, err := a.tenants.TenantExternalID(ctx, tenant)
	if err != nil {
		return "", fmt.Errorf("alchemy: tenant %s: %w", tenant, err)
	}
	if ext == "" {
		return "", fmt.Errorf("alchemy: tenant %s has no identity provider id to scope calls by", tenant)
	}
	a.ids.Store(tenant, ext)
	return ext, nil
}
