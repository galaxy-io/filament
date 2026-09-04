package server

import (
	"context"
	"errors"
	"net/http"
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
	authv1connect.AuthServiceLogoutProcedure:        true,
	authv1connect.AuthServiceRegisterProcedure:      true,
	authv1connect.AuthServiceAcceptInviteProcedure:  true,
}

// tenantFromContext returns the tenant established by the auth interceptor.
// Missing tenant context is always an authentication failure: authenticated
// requests get it from their caller and auth-disabled requests get the
// configured single tenant from the same interceptor.
func tenantFromContext(ctx context.Context) (filament.TenantID, error) {
	tenant, ok := identity.TenantFrom(ctx)
	if !ok {
		return "", connect.NewError(connect.CodeUnauthenticated, errors.New("request tenant is unavailable"))
	}
	return tenant, nil
}

// authInterceptor authenticates every RPC except the public session
// procedures, resolves the caller's tenant, and puts the caller on the
// context for handlers to scope their work by.
type authInterceptor struct {
	provider      identity.Provider
	store         filament.DataStore
	defaultTenant filament.TenantID
	// tenants caches ResolveTenant by provider organization id; a tenant
	// only needs its row minted once per boot.
	tenants sync.Map
	// users caches EnsureUser by tenant and provider subject; a user only
	// needs its row minted once per boot.
	users sync.Map
}

type userKey struct {
	tenant filament.TenantID
	userID string
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
	if i.provider == nil {
		return identity.WithTenant(ctx, i.defaultTenant), nil
	}
	if publicProcedures[procedure] {
		return ctx, nil
	}
	caller, err := i.provider.Authenticate(ctx, header)
	if err != nil {
		// A provider that says why (an outage, say) keeps its code; anything
		// else is a rejected credential.
		var cerr *connect.Error
		if errors.As(err, &cerr) {
			return nil, err
		}
		return nil, connect.NewError(connect.CodeUnauthenticated, err)
	}
	if caller.TenantExternalID == "" {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("token carries no tenant"))
	}
	tenant, ok := i.tenants.Load(caller.TenantExternalID)
	if !ok {
		resolved, err := i.store.ResolveTenant(ctx, caller.TenantExternalID, caller.TenantName)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		tenant, _ = i.tenants.LoadOrStore(caller.TenantExternalID, resolved)
	}
	caller.Tenant = tenant.(filament.TenantID)
	key := userKey{caller.Tenant, caller.UserID}
	id, ok := i.users.Load(key)
	if !ok {
		minted, err := i.store.EnsureUser(ctx, caller.Tenant, caller.UserID)
		if err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
		id, _ = i.users.LoadOrStore(key, minted)
	}
	caller.ID = id.(filament.UserID)
	return identity.WithCaller(ctx, caller), nil
}
