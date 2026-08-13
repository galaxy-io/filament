package server

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"sync"

	"connectrpc.com/connect"

	"github.com/galaxy-io/filament"
	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
	"github.com/galaxy-io/filament/identity"
)

// disabledAuth answers AuthService when no provider is configured: the
// config document reports an empty issuer so the UI renders without a
// session, and the generated base leaves every other RPC unimplemented,
// exactly as an unset metrics store leaves MetricsService.
type disabledAuth struct {
	authv1connect.UnimplementedAuthServiceHandler
}

func (disabledAuth) GetAuthConfig(_ context.Context, _ *connect.Request[authv1.GetAuthConfigRequest]) (*connect.Response[authv1.GetAuthConfigResponse], error) {
	return connect.NewResponse(&authv1.GetAuthConfigResponse{}), nil
}

// publicProcedures are the AuthService RPCs a caller reaches before holding
// a token. Everything absent from this set requires authentication, so a new
// RPC is locked down until it is deliberately listed here.
var publicProcedures = map[string]bool{
	authv1connect.AuthServiceGetAuthConfigProcedure: true,
	authv1connect.AuthServiceLoginProcedure:         true,
	authv1connect.AuthServiceRegisterProcedure:      true,
	authv1connect.AuthServiceAcceptInviteProcedure:  true,
}

// authInterceptor authenticates every RPC except the public session
// procedures, resolves the caller's tenant, and puts the caller on the
// context for handlers to scope their work by.
type authInterceptor struct {
	provider identity.Provider
	store    filament.DataStore
	// seen dedupes EnsureTenant; a tenant only needs its row once per boot.
	seen sync.Map
}

func (i *authInterceptor) WrapUnary(next connect.UnaryFunc) connect.UnaryFunc {
	return func(ctx context.Context, req connect.AnyRequest) (connect.AnyResponse, error) {
		ctx, err := i.authenticate(ctx, req.Spec().Procedure, req.Header())
		if err != nil {
			return nil, err
		}
		return next(ctx, req)
	}
}

func (i *authInterceptor) WrapStreamingHandler(next connect.StreamingHandlerFunc) connect.StreamingHandlerFunc {
	return func(ctx context.Context, conn connect.StreamingHandlerConn) error {
		ctx, err := i.authenticate(ctx, conn.Spec().Procedure, conn.RequestHeader())
		if err != nil {
			return err
		}
		return next(ctx, conn)
	}
}

func (i *authInterceptor) WrapStreamingClient(next connect.StreamingClientFunc) connect.StreamingClientFunc {
	return next
}

func (i *authInterceptor) authenticate(ctx context.Context, procedure string, header http.Header) (context.Context, error) {
	if publicProcedures[procedure] {
		return ctx, nil
	}
	bearer, ok := strings.CutPrefix(header.Get("Authorization"), "Bearer ")
	if !ok || bearer == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("missing bearer token"))
	}
	caller, err := i.provider.Authenticate(ctx, bearer)
	if err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if _, done := i.seen.Load(caller.Tenant); !done {
		if err := i.store.EnsureTenant(ctx, caller.Tenant, caller.TenantName); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		i.seen.Store(caller.Tenant, struct{}{})
	}
	return identity.WithCaller(ctx, caller), nil
}
