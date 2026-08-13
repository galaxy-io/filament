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

// errNoIdentity is what every AuthService RPC but GetAuthConfig answers when
// no provider is configured, mirroring an unset metrics store.
var errNoIdentity = errors.New("no identity provider configured")

// publicProcedures are the AuthService RPCs a caller reaches before holding
// a token. Everything absent from this set requires authentication, so a new
// RPC is locked down until it is deliberately listed here.
var publicProcedures = map[string]bool{
	authv1connect.AuthServiceGetAuthConfigProcedure: true,
	authv1connect.AuthServiceLoginProcedure:         true,
	authv1connect.AuthServiceRegisterProcedure:      true,
	authv1connect.AuthServiceAcceptInviteProcedure:  true,
}

// GetAuthConfig reports how the UI should start the sign-in flow. Unlike the
// rest of AuthService it answers with auth disabled, returning an empty
// issuer so the UI knows to render without a session.
func (a *Server) GetAuthConfig(ctx context.Context, req *connect.Request[authv1.GetAuthConfigRequest]) (*connect.Response[authv1.GetAuthConfigResponse], error) {
	if a.identity == nil {
		return connect.NewResponse(&authv1.GetAuthConfigResponse{}), nil
	}
	return a.identity.GetAuthConfig(ctx, req)
}

// Login exchanges credentials for the callback that completes the OIDC
// flow.
func (a *Server) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	if a.identity == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errNoIdentity)
	}
	return a.identity.Login(ctx, req)
}

// Register creates a tenant and its first admin.
func (a *Server) Register(ctx context.Context, req *connect.Request[authv1.RegisterRequest]) (*connect.Response[authv1.RegisterResponse], error) {
	if a.identity == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errNoIdentity)
	}
	return a.identity.Register(ctx, req)
}

// AcceptInvite redeems an invitation and sets the member's password.
func (a *Server) AcceptInvite(ctx context.Context, req *connect.Request[authv1.AcceptInviteRequest]) (*connect.Response[authv1.AcceptInviteResponse], error) {
	if a.identity == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errNoIdentity)
	}
	return a.identity.AcceptInvite(ctx, req)
}

// ListMembers returns the caller's tenant membership.
func (a *Server) ListMembers(ctx context.Context, req *connect.Request[authv1.ListMembersRequest]) (*connect.Response[authv1.ListMembersResponse], error) {
	if a.identity == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errNoIdentity)
	}
	return a.identity.ListMembers(ctx, req)
}

// InviteMember adds a teammate to the caller's tenant and returns the code
// that redeems the invitation.
func (a *Server) InviteMember(ctx context.Context, req *connect.Request[authv1.InviteMemberRequest]) (*connect.Response[authv1.InviteMemberResponse], error) {
	if a.identity == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errNoIdentity)
	}
	return a.identity.InviteMember(ctx, req)
}

// SetMemberRole reassigns a member's role.
func (a *Server) SetMemberRole(ctx context.Context, req *connect.Request[authv1.SetMemberRoleRequest]) (*connect.Response[authv1.SetMemberRoleResponse], error) {
	if a.identity == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errNoIdentity)
	}
	return a.identity.SetMemberRole(ctx, req)
}

// RemoveMember deletes a member from the caller's tenant.
func (a *Server) RemoveMember(ctx context.Context, req *connect.Request[authv1.RemoveMemberRequest]) (*connect.Response[authv1.RemoveMemberResponse], error) {
	if a.identity == nil {
		return nil, connect.NewError(connect.CodeUnimplemented, errNoIdentity)
	}
	return a.identity.RemoveMember(ctx, req)
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
