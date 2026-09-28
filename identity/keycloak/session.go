package keycloak

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"connectrpc.com/connect"

	authv1 "github.com/galaxy-io/filament/api/auth/v1"
	"github.com/galaxy-io/filament/identity"
)

// Login exchanges the credentials for an offline refresh token and hands the
// browser that token in an HttpOnly cookie. The token is the credential every
// later RPC carries: each request mints a short-lived access token from it,
// so nothing about Keycloak reaches the browser. Offline tokens outlive the
// realm's SSO session, and Logout revokes them.
const (
	sessionCookieName = "filament_sid"
	sessionLifetime   = 30 * 24 * time.Hour
	// callerCacheTTL bounds how long a resolved session skips Keycloak; role
	// changes and logouts from elsewhere take effect within it.
	callerCacheTTL = time.Minute
	// sessionScopes asks for the organization the tenant comes from and an
	// offline token the cookie can hold.
	sessionScopes = "openid organization offline_access"
)

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

// sessionFromCookie reads the refresh token off a request, reporting false
// when there is none.
func sessionFromCookie(header http.Header) (string, bool) {
	cookie, err := (&http.Request{Header: header}).Cookie(sessionCookieName)
	if err != nil || cookie.Value == "" {
		return "", false
	}
	return cookie.Value, true
}

// cachedCaller is a resolved session that skips Keycloak until expires.
type cachedCaller struct {
	caller  identity.Caller
	expires time.Time
}

// tokenResponse is the token endpoint's answer to a grant.
type tokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
}

// errGrantRejected means the token endpoint refused the grant: wrong
// credentials, a revoked token, or an account that cannot sign in.
var errGrantRejected = errors.New("grant rejected")

// grant runs one grant against the token endpoint as filament's client.
func (p *Provider) grant(ctx context.Context, form url.Values) (tokenResponse, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, p.tokenEndpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return tokenResponse{}, err
	}
	req.SetBasicAuth(p.clientID, p.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("%w: %w", errUnavailable, err)
	}
	defer func() { _ = res.Body.Close() }()
	raw, err := io.ReadAll(res.Body)
	if err != nil {
		return tokenResponse{}, fmt.Errorf("%w: %w", errUnavailable, err)
	}
	if res.StatusCode >= 500 {
		return tokenResponse{}, fmt.Errorf("%w: token endpoint status %d", errUnavailable, res.StatusCode)
	}
	if res.StatusCode != http.StatusOK {
		return tokenResponse{}, fmt.Errorf("%w: %s", errGrantRejected, reason(raw))
	}
	var token tokenResponse
	if err := json.Unmarshal(raw, &token); err != nil {
		return tokenResponse{}, fmt.Errorf("keycloak: decode token: %w", err)
	}
	return token, nil
}

// revoke invalidates a refresh token; best effort, the cookie goes away
// regardless.
func (p *Provider) revoke(ctx context.Context, refreshToken string) {
	form := url.Values{"token": {refreshToken}, "token_type_hint": {"refresh_token"}}
	endpoint := strings.TrimSuffix(p.tokenEndpoint, "/token") + "/revoke"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, strings.NewReader(form.Encode()))
	if err != nil {
		return
	}
	req.SetBasicAuth(p.clientID, p.clientSecret)
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if res, err := http.DefaultClient.Do(req); err == nil {
		_ = res.Body.Close()
	}
}

// authenticateSession resolves the caller behind a session cookie by minting
// an access token from its refresh token and reading that token's claims.
func (p *Provider) authenticateSession(ctx context.Context, refreshToken string) (identity.Caller, error) {
	now := time.Now()
	if cached, ok := p.callers.Load(refreshToken); ok && now.Before(cached.(cachedCaller).expires) {
		return cached.(cachedCaller).caller, nil
	}
	token, err := p.grant(ctx, url.Values{"grant_type": {"refresh_token"}, "refresh_token": {refreshToken}})
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return identity.Caller{}, connect.NewError(connect.CodeUnavailable, err)
		}
		return identity.Caller{}, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid session"))
	}
	caller, err := p.authenticateToken(ctx, token.AccessToken)
	if err != nil {
		return identity.Caller{}, err
	}
	p.rememberCaller(refreshToken, caller, now)
	return caller, nil
}

// rememberCaller caches a resolved session and drops any entries that have
// expired; logins are rare enough that the sweep costs nothing.
func (p *Provider) rememberCaller(key string, caller identity.Caller, now time.Time) {
	p.callers.Range(func(k, value any) bool {
		if !now.Before(value.(cachedCaller).expires) {
			p.callers.Delete(k)
		}
		return true
	})
	p.callers.Store(key, cachedCaller{caller: caller, expires: now.Add(callerCacheTTL)})
}

// Login checks the credentials with the direct access grant and answers with
// the cookie that carries the session. Proxying keeps Keycloak off the
// browser's origin and the flow in one place.
func (p *Provider) Login(ctx context.Context, req *connect.Request[authv1.LoginRequest]) (*connect.Response[authv1.LoginResponse], error) {
	m := req.Msg
	if m.GetLoginName() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("login_name and password are required"))
	}
	token, err := p.grant(ctx, url.Values{
		"grant_type": {"password"},
		"username":   {m.GetLoginName()},
		"password":   {m.GetPassword()},
		"scope":      {sessionScopes},
	})
	if err != nil {
		if errors.Is(err, errUnavailable) {
			return nil, connect.NewError(connect.CodeUnavailable, err)
		}
		// Unknown users and wrong passwords both land here; neither should
		// tell the caller which it was.
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("invalid credentials"))
	}
	res := connect.NewResponse(&authv1.LoginResponse{})
	res.Header().Add("Set-Cookie", p.sessionCookie(token.RefreshToken, int(sessionLifetime.Seconds())).String())
	return res, nil
}

// Logout revokes the session's token and clears its cookie.
func (p *Provider) Logout(ctx context.Context, req *connect.Request[authv1.LogoutRequest]) (*connect.Response[authv1.LogoutResponse], error) {
	if session, ok := sessionFromCookie(req.Header()); ok {
		p.callers.Delete(session)
		p.revoke(ctx, session)
	}
	res := connect.NewResponse(&authv1.LogoutResponse{})
	res.Header().Add("Set-Cookie", p.sessionCookie("", -1).String())
	return res, nil
}

// GetSession describes the caller behind the request for the UI's own
// account surfaces. Service accounts have no human profile and answer with
// their id alone.
func (p *Provider) GetSession(ctx context.Context, _ *connect.Request[authv1.GetSessionRequest]) (*connect.Response[authv1.GetSessionResponse], error) {
	caller, ok := identity.CallerFrom(ctx)
	if !ok {
		return nil, connect.NewError(connect.CodeUnauthenticated, errors.New("unauthenticated"))
	}
	user, err := p.admin.getUser(ctx, caller.UserID)
	if err != nil {
		return nil, rpcError(err)
	}
	return connect.NewResponse(&authv1.GetSessionResponse{
		UserId: caller.UserID,
		Name:   displayName(user),
		Email:  user.Email,
	}), nil
}

func displayName(user userRep) string {
	return strings.TrimSpace(user.FirstName + " " + user.LastName)
}

// aliasChars is everything an organization alias cannot keep.
var aliasChars = regexp.MustCompile(`[^a-z0-9]+`)

// alias derives an organization's alias from its name. Keycloak fixes the
// alias at creation, which is what makes it usable as the tenant id.
func alias(name string) string {
	a := strings.Trim(aliasChars.ReplaceAllString(strings.ToLower(name), "-"), "-")
	if a == "" {
		return "org"
	}
	return a
}

// Register creates an organization and its first admin: filament's
// self-service signup. The organization becomes the tenant.
func (p *Provider) Register(ctx context.Context, req *connect.Request[authv1.RegisterRequest]) (*connect.Response[authv1.RegisterResponse], error) {
	m := req.Msg
	if m.GetOrgName() == "" || m.GetGivenName() == "" || m.GetFamilyName() == "" || m.GetEmail() == "" || m.GetPassword() == "" {
		return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("org_name, given_name, family_name, email, and password are required"))
	}
	orgID, err := p.admin.createOrganization(ctx, organizationRep{Name: m.GetOrgName(), Alias: alias(m.GetOrgName())})
	if err != nil {
		if isStatus(err, http.StatusConflict) {
			return nil, connect.NewError(connect.CodeInvalidArgument, errors.New("an organization with that name already exists"))
		}
		return nil, rpcError(err)
	}
	// Filament sends no mail, so the founder's address starts verified and
	// they can sign in immediately.
	userID, err := p.admin.createUser(ctx, userRep{
		Username:      m.GetEmail(),
		Email:         m.GetEmail(),
		FirstName:     m.GetGivenName(),
		LastName:      m.GetFamilyName(),
		Enabled:       true,
		EmailVerified: true,
		Credentials:   []credentialRep{{Type: "password", Value: m.GetPassword()}},
	})
	if err != nil {
		// Duplicate emails and password-policy failures both land here
		// with Keycloak's reason attached; the organization goes with them
		// so the name is free for a retry.
		_ = p.admin.deleteOrganization(ctx, orgID)
		return nil, rpcError(err)
	}
	if err := p.admin.addMember(ctx, orgID, userID); err != nil {
		return nil, rpcError(p.discardUser(ctx, userID, err))
	}
	// The founder administers the tenant they just created.
	if err := p.grantRole(ctx, userID, identity.RoleKey(authv1.Role_ROLE_ADMIN)); err != nil {
		return nil, rpcError(p.discardUser(ctx, userID, err))
	}
	return connect.NewResponse(&authv1.RegisterResponse{TenantId: alias(m.GetOrgName())}), nil
}
