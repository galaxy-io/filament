package zitadel

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	authorizationv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/authorization/v2"
	filterv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/filter/v2"
	orgv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/org/v2"
	sessionv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/session/v2"
	userv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
	"google.golang.org/protobuf/types/known/durationpb"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

// Login creates a Zitadel session and hands the browser its id and token in
// an HttpOnly cookie. That cookie is the credential every later RPC carries,
// so nothing about Zitadel reaches the browser. The cookie and the session
// share a lifetime, so both expire together.
const (
	sessionCookieName = "filament_sid"
	sessionLifetime   = 30 * 24 * time.Hour
	// callerCacheTTL bounds how long a resolved session skips Zitadel; role
	// changes and logouts from elsewhere take effect within it.
	callerCacheTTL = time.Minute
)

// sessionRef identifies a Zitadel session together with the token that
// authorizes acting on it.
type sessionRef struct {
	id    string
	token string
}

func (s sessionRef) String() string { return s.id + "." + s.token }

// sessionCookie builds the session cookie; an empty value with a negative
// maxAge clears it.
func (p *Provider) sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{ //nolint:gosec // Secure follows the UI origin's scheme so plain-http local dev keeps its cookie
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   p.secureCookies,
		SameSite: http.SameSiteLaxMode,
	}
}

// sessionFromCookie reads the session off a request, reporting false when
// there is none. The token is required: the machine user could read any
// session by id alone, so a cookie without one must never be honored.
func sessionFromCookie(header http.Header) (sessionRef, bool) {
	cookie, err := (&http.Request{Header: header}).Cookie(sessionCookieName)
	if err != nil {
		return sessionRef{}, false
	}
	id, token, ok := strings.Cut(cookie.Value, ".")
	if !ok || id == "" || token == "" {
		return sessionRef{}, false
	}
	return sessionRef{id: id, token: token}, true
}

// cachedCaller is a resolved session that skips Zitadel until expires.
type cachedCaller struct {
	caller  identity.Caller
	expires time.Time
}

// authenticateSession resolves the caller behind a session cookie: Zitadel
// validates the token, the session names the user and organization, and an
// authorization lookup supplies the roles.
func (p *Provider) authenticateSession(ctx context.Context, ref sessionRef) (identity.Caller, error) {
	now := time.Now()
	if cached, ok := p.callers.Load(ref); ok && now.Before(cached.(cachedCaller).expires) {
		return cached.(cachedCaller).caller, nil
	}
	found, err := p.api.SessionServiceV2().GetSession(ctx, &sessionv2.GetSessionRequest{
		SessionId:    ref.id,
		SessionToken: &ref.token,
	})
	if err != nil {
		if isUnavailable(err) {
			return identity.Caller{}, connect.NewError(connect.CodeUnavailable, err)
		}
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid session"))
	}
	session := found.GetSession()
	if expiry := session.GetExpirationDate(); expiry != nil && !now.Before(expiry.AsTime()) {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("session expired"))
	}
	user := session.GetFactors().GetUser()
	if user.GetId() == "" || user.GetOrganizationId() == "" {
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("session carries no user"))
	}
	roles, err := p.userRoles(ctx, user.GetId())
	if err != nil {
		if isUnavailable(err) {
			return identity.Caller{}, connect.NewError(connect.CodeUnavailable, err)
		}
		return identity.Caller{}, connect.NewError(connect.CodeInternal, err)
	}
	caller := identity.Caller{
		UserID:           user.GetId(),
		TenantExternalID: user.GetOrganizationId(),
		Roles:            roles,
	}
	p.rememberCaller(ref, caller, now)
	return caller, nil
}

// rememberCaller caches a resolved session and drops any entries that have
// expired; logins are rare enough that the sweep costs nothing.
func (p *Provider) rememberCaller(ref sessionRef, caller identity.Caller, now time.Time) {
	p.callers.Range(func(key, value any) bool {
		if !now.Before(value.(cachedCaller).expires) {
			p.callers.Delete(key)
		}
		return true
	})
	p.callers.Store(ref, cachedCaller{caller: caller, expires: now.Add(callerCacheTTL)})
}

// userRoles returns the filament roles granted to one user in the project.
func (p *Provider) userRoles(ctx context.Context, userID string) ([]authv1.Role, error) {
	granted, err := p.api.AuthorizationServiceV2().ListAuthorizations(ctx, &authorizationv2.ListAuthorizationsRequest{
		Filters: []*authorizationv2.AuthorizationsSearchFilter{
			{Filter: &authorizationv2.AuthorizationsSearchFilter_ProjectId{ProjectId: &filterv2.IDFilter{Id: p.project.id}}},
			{Filter: &authorizationv2.AuthorizationsSearchFilter_InUserIds{InUserIds: &filterv2.InIDsFilter{Ids: []string{userID}}}},
		},
	})
	if err != nil {
		return nil, err
	}
	var roles []authv1.Role
	for _, authorization := range granted.GetAuthorizations() {
		if role := grantedRole(authorization); role != authv1.Role_ROLE_UNSPECIFIED {
			roles = append(roles, role)
		}
	}
	return roles, nil
}

// Login checks the credentials as a Zitadel session and answers with the
// cookie that carries it. Proxying keeps Zitadel off the browser's origin
// and the flow in one place.
func (p *Provider) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	m := req.Msg
	if m.GetLoginName() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("login_name and password are required"))
	}

	created, err := p.api.SessionServiceV2().CreateSession(ctx, &sessionv2.CreateSessionRequest{
		Checks: &sessionv2.Checks{
			User: &sessionv2.CheckUser{Search: &sessionv2.CheckUser_LoginName{LoginName: m.GetLoginName()}},
			Password: &sessionv2.CheckPassword{
				Password: m.GetPassword(),
			},
		},
		Lifetime: durationpb.New(sessionLifetime),
	})
	if err != nil {
		if isUnavailable(err) {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		// Unknown users and wrong passwords both land here; neither should
		// tell the caller which it was.
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	session := sessionRef{id: created.GetSessionId(), token: created.GetSessionToken()}
	res := connect.NewResponse(&authv1.LoginResponse{})
	res.Header().Add("Set-Cookie", p.sessionCookie(session.String(), int(sessionLifetime.Seconds())).String())
	return res, nil
}

// Logout ends the session and clears its cookie. Terminating the session in
// Zitadel is best effort: the cookie goes away regardless.
func (p *Provider) Logout(ctx context.Context, req *connect.Request[authv1.LogoutRequest]) (*connect.Response[authv1.LogoutResponse], error) {
	if session, ok := sessionFromCookie(req.Header()); ok {
		p.callers.Delete(session)
		_, _ = p.api.SessionServiceV2().DeleteSession(ctx, &sessionv2.DeleteSessionRequest{
			SessionId:    session.id,
			SessionToken: &session.token,
		})
	}
	res := connect.NewResponse(&authv1.LogoutResponse{})
	res.Header().Add("Set-Cookie", p.sessionCookie("", -1).String())
	return res, nil
}

// GetSession describes the caller behind the request for the UI's own
// account surfaces. Machine users have no human profile and answer with
// their id alone.
func (p *Provider) GetSession(ctx context.Context, _ *connect.Request[authv1.GetSessionRequest]) (*connect.Response[authv1.GetSessionResponse], error) {
	caller, ok := identity.CallerFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	found, err := p.api.UserServiceV2().GetUserByID(ctx, &userv2.GetUserByIDRequest{UserId: caller.UserID})
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	human := found.GetUser().GetHuman()
	return connect.NewResponse(&authv1.GetSessionResponse{
		UserId:    caller.UserID,
		Name:      human.GetProfile().GetDisplayName(),
		Email:     human.GetEmail().GetEmail(),
		AvatarUrl: human.GetProfile().GetAvatarUrl(),
	}), nil
}

// Register creates an organization and its first admin: filament's
// self-service signup. The organization becomes the tenant.
func (p *Provider) Register(ctx context.Context, req *connect.Request[authv1.RegisterRequest]) (*connect.Response[authv1.RegisterResponse], error) {
	m := req.Msg
	if m.GetOrgName() == "" || m.GetGivenName() == "" || m.GetFamilyName() == "" || m.GetEmail() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("org_name, given_name, family_name, email, and password are required"))
	}

	created, err := p.api.OrganizationServiceV2().AddOrganization(ctx, &orgv2.AddOrganizationRequest{
		Name: m.GetOrgName(),
		Admins: []*orgv2.AddOrganizationRequest_Admin{{
			UserType: &orgv2.AddOrganizationRequest_Admin_Human{
				Human: &userv2.AddHumanUserRequest{
					Profile: &userv2.SetHumanProfile{
						GivenName:  m.GetGivenName(),
						FamilyName: m.GetFamilyName(),
					},
					// Filament sends no mail, so the founder's address
					// starts verified and they can sign in immediately.
					Email: &userv2.SetHumanEmail{
						Email:        m.GetEmail(),
						Verification: &userv2.SetHumanEmail_IsVerified{IsVerified: true},
					},
					PasswordType: &userv2.AddHumanUserRequest_Password{
						Password: &userv2.Password{Password: m.GetPassword()},
					},
				},
			},
		}},
	})
	if err != nil {
		// Duplicate names and password-policy failures both land here with
		// Zitadel's reason attached.
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}

	// The founder administers the tenant they just created.
	orgID := created.GetOrganizationId()
	adminRole := identity.RoleKey(authv1.Role_ROLE_ADMIN)
	for _, admin := range created.GetCreatedAdmins() {
		if err := p.grantRole(ctx, orgID, admin.GetUserId(), adminRole); err != nil {
			return nil, connect.NewError(connect.CodeInternal, err)
		}
	}
	return connect.NewResponse(&authv1.RegisterResponse{TenantId: orgID}), nil
}

// AcceptInvite redeems an invitation and sets the member's password. The
// code is the credential, which is why this RPC is public.
func (p *Provider) AcceptInvite(ctx context.Context, req *connect.Request[authv1.AcceptInviteRequest]) (*connect.Response[authv1.AcceptInviteResponse], error) {
	m := req.Msg
	if m.GetUserId() == "" || m.GetCode() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("user_id, code, and password are required"))
	}
	if _, err := p.api.UserServiceV2().VerifyInviteCode(ctx, &userv2.VerifyInviteCodeRequest{
		UserId:           m.GetUserId(),
		VerificationCode: m.GetCode(),
	}); err != nil {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid or expired invite"))
	}
	if _, err := p.api.UserServiceV2().UpdateUser(ctx, &userv2.UpdateUserRequest{
		UserId: m.GetUserId(),
		UserType: &userv2.UpdateUserRequest_Human_{
			Human: &userv2.UpdateUserRequest_Human{
				Password: &userv2.SetPassword{
					PasswordType: &userv2.SetPassword_Password{
						Password: &userv2.Password{Password: m.GetPassword(), ChangeRequired: false},
					},
				},
			},
		},
	}); err != nil {
		// Password-policy failures surface with Zitadel's reason.
		return nil, connect.NewError(connect.CodeInvalidArgument, err)
	}
	return connect.NewResponse(&authv1.AcceptInviteResponse{}), nil
}
