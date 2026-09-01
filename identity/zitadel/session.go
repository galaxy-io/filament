package zitadel

import (
	"context"
	"errors"
	"net/http"
	"strings"
	"time"

	"connectrpc.com/connect"
	oidcv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/oidc/v2"
	orgv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/org/v2"
	sessionv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/session/v2"
	userv2 "github.com/zitadel/zitadel-go/v3/pkg/client/zitadel/user/v2"
	"google.golang.org/protobuf/types/known/durationpb"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/api/auth/v1/authv1connect"
	"github.com/galaxy-io/filament/identity"
)

// The session Login creates is remembered in a cookie so a new tab can
// finish its own authorization without credentials. The cookie is HttpOnly
// and scoped to AuthService, the only place it is read; it and the Zitadel
// session share a lifetime, so both expire together.
const (
	sessionCookieName = "filament_session"
	sessionCookiePath = "/" + authv1connect.AuthServiceName
	sessionLifetime   = 30 * 24 * time.Hour
)

// sessionRef identifies a Zitadel session together with the token that
// authorizes acting on it.
type sessionRef struct {
	id    string
	token string
}

func (s sessionRef) String() string { return s.id + "." + s.token }

// sessionCookie builds the remembered-session cookie; an empty value with a
// negative maxAge clears it.
func (p *Provider) sessionCookie(value string, maxAge int) *http.Cookie {
	return &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     sessionCookiePath,
		MaxAge:   maxAge,
		HttpOnly: true,
		Secure:   p.secureCookies,
		SameSite: http.SameSiteLaxMode,
	}
}

// sessionFromCookie reads the remembered session off a request, reporting
// false when there is none.
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

// createCallback finalizes a pending auth request with session and returns
// the URL that completes the flow in the browser.
func (p *Provider) createCallback(ctx context.Context, authRequestID string, session sessionRef) (string, error) {
	callback, err := p.api.OIDCServiceV2().CreateCallback(ctx, &oidcv2.CreateCallbackRequest{
		AuthRequestId: authRequestID,
		CallbackKind: &oidcv2.CreateCallbackRequest_Session{
			Session: &oidcv2.Session{
				SessionId:    session.id,
				SessionToken: session.token,
			},
		},
	})
	if err != nil {
		return "", err
	}
	return callback.GetCallbackUrl(), nil
}

// Login exchanges credentials for an OIDC callback: check them as a
// session, finalize the pending auth request with it, and hand the browser
// the URL that completes the flow. Proxying keeps Zitadel off the browser's
// origin and the flow in one place. The session is also remembered in a
// cookie for ResumeLogin.
func (p *Provider) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	m := req.Msg
	if m.GetAuthRequestId() == "" || m.GetLoginName() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("auth_request_id, login_name, and password are required"))
	}

	loginName := m.GetLoginName()
	created, err := p.api.SessionServiceV2().CreateSession(ctx, &sessionv2.CreateSessionRequest{
		Checks: &sessionv2.Checks{
			User: &sessionv2.CheckUser{Search: &sessionv2.CheckUser_LoginName{LoginName: loginName}},
			Password: &sessionv2.CheckPassword{
				Password: m.GetPassword(),
			},
		},
		Lifetime: durationpb.New(sessionLifetime),
	})
	if err != nil {
		// Unknown users and wrong passwords both land here; neither should
		// tell the caller which it was.
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}

	session := sessionRef{id: created.GetSessionId(), token: created.GetSessionToken()}
	callbackURL, err := p.createCallback(ctx, m.GetAuthRequestId(), session)
	if err != nil {
		return nil, connect.NewError(connect.CodeInternal, err)
	}
	res := connect.NewResponse(&authv1.LoginResponse{CallbackUrl: callbackURL})
	res.Header().Add("Set-Cookie", p.sessionCookie(session.String(), int(sessionLifetime.Seconds())).String())
	return res, nil
}

// ResumeLogin finalizes a pending auth request with the remembered session,
// which is how a new tab signs in without a form. A session Zitadel no
// longer accepts takes its cookie with it, so the next attempt goes
// straight to credentials.
func (p *Provider) ResumeLogin(ctx context.Context, req *connect.Request[authv1.ResumeLoginRequest]) (*connect.Response[authv1.ResumeLoginResponse], error) {
	if req.Msg.GetAuthRequestId() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("auth_request_id is required"))
	}
	session, ok := sessionFromCookie(req.Header())
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("no remembered session"))
	}
	callbackURL, err := p.createCallback(ctx, req.Msg.GetAuthRequestId(), session)
	if err != nil {
		cerr := connect.NewError(connect.CodeUnauthenticated, errors.New("remembered session is no longer valid"))
		cerr.Meta().Add("Set-Cookie", p.sessionCookie("", -1).String())
		return nil, cerr
	}
	return connect.NewResponse(&authv1.ResumeLoginResponse{CallbackUrl: callbackURL}), nil
}

// Logout ends the remembered session and clears its cookie. Terminating the
// session in Zitadel is best effort: the cookie goes away regardless, and
// the browser follows up with the provider's end-session redirect.
func (p *Provider) Logout(ctx context.Context, req *connect.Request[authv1.LogoutRequest]) (*connect.Response[authv1.LogoutResponse], error) {
	if session, ok := sessionFromCookie(req.Header()); ok {
		_, _ = p.api.SessionServiceV2().DeleteSession(ctx, &sessionv2.DeleteSessionRequest{
			SessionId:    session.id,
			SessionToken: &session.token,
		})
	}
	res := connect.NewResponse(&authv1.LogoutResponse{})
	res.Header().Add("Set-Cookie", p.sessionCookie("", -1).String())
	return res, nil
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
