package auth

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// loginRequest is what the filament login page posts: the auth request id
// Zitadel appended to the login redirect, plus the user's credentials.
type loginRequest struct {
	AuthRequestID string `json:"authRequestId"`
	LoginName     string `json:"loginName"`
	Password      string `json:"password"`
}

// LoginHandler proxies the filament login page to Zitadel's session API:
// verify the credentials as a session, finalize the pending auth request
// with it, and hand back the OIDC callback URL the browser must follow.
// Proxying keeps Zitadel off the browser's origin and the flow in one place.
func (c *Config) LoginHandler() http.Handler {
	client := &http.Client{Timeout: 10 * time.Second}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req loginRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.AuthRequestID == "" || req.LoginName == "" || req.Password == "" {
			http.Error(w, "authRequestId, loginName, and password are required", http.StatusBadRequest)
			return
		}

		session := map[string]any{
			"checks": map[string]any{
				"user":     map[string]any{"loginName": req.LoginName},
				"password": map[string]any{"password": req.Password},
			},
		}
		var sess struct {
			SessionID    string `json:"sessionId"`
			SessionToken string `json:"sessionToken"`
		}
		if err := c.postZitadel(client, r, "/v2/sessions", session, &sess); err != nil {
			// Zitadel rejects wrong users and wrong passwords alike here;
			// surface both as a credential failure.
			http.Error(w, "invalid credentials", http.StatusUnauthorized)
			return
		}

		finalize := map[string]any{
			"session": map[string]any{"sessionId": sess.SessionID, "sessionToken": sess.SessionToken},
		}
		var out struct {
			CallbackURL string `json:"callbackUrl"`
		}
		if err := c.postZitadel(client, r, "/v2/oidc/auth_requests/"+req.AuthRequestID, finalize, &out); err != nil {
			http.Error(w, "finalize auth request: "+err.Error(), http.StatusBadGateway)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(out)
	})
}

func (c *Config) postZitadel(client *http.Client, r *http.Request, path string, body, out any) error {
	return c.doZitadelOrg(client, r, http.MethodPost, "", path, body, out)
}

func (c *Config) postZitadelOrg(client *http.Client, r *http.Request, orgID, path string, body, out any) error {
	return c.doZitadelOrg(client, r, http.MethodPost, orgID, path, body, out)
}

func (c *Config) putZitadelOrg(client *http.Client, r *http.Request, orgID, path string, body any) error {
	return c.doZitadelOrg(client, r, http.MethodPut, orgID, path, body, &struct{}{})
}

func (c *Config) deleteZitadel(client *http.Client, r *http.Request, path string) error {
	return c.doZitadelOrg(client, r, http.MethodDelete, "", path, nil, &struct{}{})
}

// doZitadelOrg calls the Zitadel API with the PAT; a non-empty orgID scopes
// the call to that org via the x-zitadel-orgid header.
func (c *Config) doZitadelOrg(client *http.Client, r *http.Request, method, orgID, path string, body, out any) error {
	var reader io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		reader = bytes.NewReader(buf)
	}
	// The issuer is operator-configured (AUTH_ISSUER) and paths are code
	// constants; nothing caller-controlled reaches the URL.
	req, err := http.NewRequestWithContext(r.Context(), method, c.Issuer+path, reader) //nolint:gosec // see above
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.pat)
	req.Header.Set("Content-Type", "application/json")
	if orgID != "" {
		req.Header.Set("x-zitadel-orgid", orgID)
	}
	resp, err := client.Do(req) //nolint:gosec // same operator-configured target
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
