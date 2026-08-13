package auth

import (
	"context"
	"net/http"
	"sync"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
)

// Interceptor returns a Connect interceptor that requires a valid bearer
// token on every RPC, resolves the token's org to a tenant (creating the
// tenant row on first sight), and stashes it on the context.
func (c *Config) Interceptor(store filament.DataStore) connect.Interceptor {
	return &interceptor{cfg: c, store: store}
}

type interceptor struct {
	cfg   *Config
	store filament.DataStore
	seen  sync.Map // tenant id -> struct{}, EnsureTenant already ran
}

func (i *interceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authenticate(ctx, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *interceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authenticate(ctx, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *interceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *interceptor) authenticate(ctx context.Context, header http.Header) (context.Context, error) {
	caller, err := i.cfg.verifyBearer(ctx, header)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	tenant := filament.TenantID(caller.OrgID)
	if _, done := i.seen.Load(tenant); !done {
		if err := i.store.EnsureTenant(ctx, tenant, caller.OrgName); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		i.seen.Store(tenant, struct{}{})
	}
	return filament.WithRoles(filament.WithTenant(ctx, tenant), caller.Roles), nil
}
