package auth

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

const (
	projectName = "filament"
	appName     = "filament-ui"
)

// Project roles. The org founder is admin; invites hand out creator or
// viewer.
const (
	RoleAdmin   = "admin"
	RoleCreator = "creator"
	RoleViewer  = "viewer"
)

var projectRoles = []string{RoleAdmin, RoleCreator, RoleViewer}

// mgmtClient is the boot-time Zitadel management API client.
type mgmtClient struct {
	issuer string
	pat    string
	http   *http.Client
}

// ensureApp makes sure the filament project, its roles, and its SPA
// application exist in Zitadel and returns the project (with its owner
// org) and client id. Every step is idempotent, so each boot converges the
// instance on the same state.
func ensureApp(ctx context.Context, issuer, pat, origin string) (foundProject, string, error) {
	c := &mgmtClient{issuer: issuer, pat: pat, http: &http.Client{Timeout: 10 * time.Second}}

	// A fresh instance forces every app onto the built-in v2 login; per-app
	// routing (to filament's login page) only applies with the force off.
	if err := c.do(ctx, http.MethodPut, "/v2/features/instance", map[string]any{"loginV2": map[string]any{"required": false}}, &struct{}{}); err != nil {
		return foundProject{}, "", err
	}
	// Finalizing auth requests needs IAM_LOGIN_CLIENT; the first-instance
	// machine user only gets IAM_OWNER.
	if err := c.ensureLoginClientRole(ctx); err != nil {
		return foundProject{}, "", err
	}

	project, err := c.findProject(ctx)
	if err != nil {
		return foundProject{}, "", err
	}
	if project.id == "" {
		if project, err = c.createProject(ctx); err != nil {
			return foundProject{}, "", err
		}
	}
	if err := c.ensureProjectRoles(ctx, project.id); err != nil {
		return foundProject{}, "", err
	}
	app, err := c.findApp(ctx, project.id)
	if err != nil {
		return foundProject{}, "", err
	}
	if app.id == "" {
		clientID, err := c.createApp(ctx, project.id, origin)
		return project, clientID, err
	}
	// Converge a pre-existing app on the current desired config; the login
	// base URI in particular must track AUTH_UI_ORIGIN.
	if app.loginBase != origin {
		if err := c.updateApp(ctx, project.id, app.id, origin); err != nil {
			return foundProject{}, "", err
		}
	}
	return project, app.clientID, nil
}

// ensureProjectRoles adds any missing filament roles to the project.
func (c *mgmtClient) ensureProjectRoles(ctx context.Context, projectID string) error {
	var out struct {
		Result []struct {
			Key string `json:"key"`
		} `json:"result"`
	}
	if err := c.post(ctx, "/management/v1/projects/"+projectID+"/roles/_search", map[string]any{}, &out); err != nil {
		return err
	}
	existing := map[string]bool{}
	for _, role := range out.Result {
		existing[role.Key] = true
	}
	for _, key := range projectRoles {
		if existing[key] {
			continue
		}
		body := map[string]any{"roleKey": key, "displayName": key}
		if err := c.post(ctx, "/management/v1/projects/"+projectID+"/roles", body, &struct{}{}); err != nil {
			return err
		}
	}
	return nil
}

// ensureLoginClientRole grants the PAT's own user IAM_LOGIN_CLIENT alongside
// its existing instance roles, once.
func (c *mgmtClient) ensureLoginClientRole(ctx context.Context) error {
	var me struct {
		User struct {
			ID string `json:"id"`
		} `json:"user"`
	}
	if err := c.do(ctx, http.MethodGet, "/auth/v1/users/me", nil, &me); err != nil {
		return err
	}
	body := map[string]any{
		"queries": []any{map[string]any{
			"userIdQuery": map[string]any{"userId": me.User.ID},
		}},
	}
	var members struct {
		Result []struct {
			Roles []string `json:"roles"`
		} `json:"result"`
	}
	if err := c.post(ctx, "/admin/v1/members/_search", body, &members); err != nil {
		return err
	}
	if len(members.Result) == 0 {
		return c.post(ctx, "/admin/v1/members", map[string]any{"userId": me.User.ID, "roles": []string{"IAM_LOGIN_CLIENT"}}, &struct{}{})
	}
	roles := members.Result[0].Roles
	for _, r := range roles {
		if r == "IAM_LOGIN_CLIENT" {
			return nil
		}
	}
	return c.do(ctx, http.MethodPut, "/admin/v1/members/"+me.User.ID, map[string]any{"roles": append(roles, "IAM_LOGIN_CLIENT")}, &struct{}{})
}

func (c *mgmtClient) post(ctx context.Context, path string, body, out any) error {
	return c.do(ctx, http.MethodPost, path, body, out)
}

func (c *mgmtClient) do(ctx context.Context, method, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.issuer+path, reader)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.http.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		var e struct {
			Message string `json:"message"`
		}
		_ = json.NewDecoder(resp.Body).Decode(&e)
		return fmt.Errorf("zitadel %s: %s (%s)", path, resp.Status, e.Message)
	}
	return json.NewDecoder(resp.Body).Decode(out)
}

func (c *mgmtClient) findProject(ctx context.Context) (foundProject, error) {
	body := map[string]any{
		"queries": []any{map[string]any{
			"nameQuery": map[string]any{"name": projectName, "method": "TEXT_QUERY_METHOD_EQUALS"},
		}},
	}
	var out struct {
		Result []struct {
			ID      string `json:"id"`
			Details struct {
				ResourceOwner string `json:"resourceOwner"`
			} `json:"details"`
		} `json:"result"`
	}
	if err := c.post(ctx, "/management/v1/projects/_search", body, &out); err != nil {
		return foundProject{}, err
	}
	if len(out.Result) == 0 {
		return foundProject{}, nil
	}
	return foundProject{id: out.Result[0].ID, ownerOrgID: out.Result[0].Details.ResourceOwner}, nil
}

type foundProject struct {
	id         string
	ownerOrgID string
}

func (c *mgmtClient) createProject(ctx context.Context) (foundProject, error) {
	var out struct {
		ID      string `json:"id"`
		Details struct {
			ResourceOwner string `json:"resourceOwner"`
		} `json:"details"`
	}
	// projectRoleAssertion puts granted roles in every token for this
	// project's apps.
	body := map[string]any{"name": projectName, "projectRoleAssertion": true}
	if err := c.post(ctx, "/management/v1/projects", body, &out); err != nil {
		return foundProject{}, err
	}
	return foundProject{id: out.ID, ownerOrgID: out.Details.ResourceOwner}, nil
}

type foundApp struct {
	id        string
	clientID  string
	loginBase string
}

func (c *mgmtClient) findApp(ctx context.Context, projectID string) (foundApp, error) {
	body := map[string]any{
		"queries": []any{map[string]any{
			"nameQuery": map[string]any{"name": appName, "method": "TEXT_QUERY_METHOD_EQUALS"},
		}},
	}
	var out struct {
		Result []struct {
			ID         string `json:"id"`
			OIDCConfig struct {
				ClientID     string `json:"clientId"`
				LoginVersion struct {
					LoginV2 struct {
						BaseURI string `json:"baseUri"`
					} `json:"loginV2"`
				} `json:"loginVersion"`
			} `json:"oidcConfig"`
		} `json:"result"`
	}
	if err := c.post(ctx, "/management/v1/projects/"+projectID+"/apps/_search", body, &out); err != nil {
		return foundApp{}, err
	}
	if len(out.Result) == 0 {
		return foundApp{}, nil
	}
	found := out.Result[0]
	return foundApp{id: found.ID, clientID: found.OIDCConfig.ClientID, loginBase: found.OIDCConfig.LoginVersion.LoginV2.BaseURI}, nil
}

// createApp registers the SPA: PKCE without a client secret, code flow only,
// JWT access tokens so the API can verify them against JWKS without an
// introspection round trip. devMode permits http:// redirect URIs.
// loginVersion routes the authorize endpoint to filament's own login page
// instead of Zitadel's hosted one.
func (c *mgmtClient) createApp(ctx context.Context, projectID, origin string) (string, error) {
	body := oidcAppConfig(origin)
	body["name"] = appName
	var out struct {
		ClientID string `json:"clientId"`
	}
	if err := c.post(ctx, "/management/v1/projects/"+projectID+"/apps/oidc", body, &out); err != nil {
		return "", err
	}
	return out.ClientID, nil
}

func (c *mgmtClient) updateApp(ctx context.Context, projectID, appID, origin string) error {
	var out struct{}
	return c.do(ctx, http.MethodPut, "/management/v1/projects/"+projectID+"/apps/"+appID+"/oidc_config", oidcAppConfig(origin), &out)
}

func oidcAppConfig(origin string) map[string]any {
	//nolint:gosec // OIDC protocol constants, not credentials
	return map[string]any{
		"appType":                "OIDC_APP_TYPE_USER_AGENT",
		"authMethodType":         "OIDC_AUTH_METHOD_TYPE_NONE",
		"grantTypes":             []string{"OIDC_GRANT_TYPE_AUTHORIZATION_CODE", "OIDC_GRANT_TYPE_REFRESH_TOKEN"},
		"responseTypes":          []string{"OIDC_RESPONSE_TYPE_CODE"},
		"redirectUris":           []string{origin + "/auth/callback"},
		"postLogoutRedirectUris": []string{origin},
		"accessTokenType":        "OIDC_TOKEN_TYPE_JWT",
		"devMode":                true,
		// Roles land in the access token so the API can authorize without
		// an extra lookup.
		"accessTokenRoleAssertion": true,
		// Zitadel appends /login to the base URI on redirect.
		"loginVersion": map[string]any{"loginV2": map[string]any{"baseUri": origin}},
	}
}
