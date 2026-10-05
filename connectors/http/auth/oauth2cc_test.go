package auth

import (
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament/connectors/http/template"
)

func TestOAuthGrantsCacheAndRenewal(t *testing.T) {
	for _, grant := range []string{"client_credentials", "refresh_token"} {
		t.Run(grant, func(t *testing.T) {
			var calls atomic.Int32
			api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if err := r.ParseForm(); err != nil {
					t.Error(err)
				}
				if r.Method != "POST" || r.Form.Get("grant_type") != grant || r.Form.Get("client_id") != "client" || r.Form.Get("client_secret") != "secret" || r.Form.Get("scope") != "read" || r.Form.Get("soid") != "org" {
					t.Error("unexpected token request")
				}
				wantRefresh := ""
				if grant == "refresh_token" {
					wantRefresh = "refresh"
				}
				if r.Form.Get("refresh_token") != wantRefresh {
					t.Error("unexpected refresh token")
				}
				fmt.Fprintf(w, `{"access_token":"token-%d","expires_in":3600}`, calls.Add(1))
			}))
			t.Cleanup(api.Close)
			params := map[string]any{
				"token_url": api.URL, "client_id": "{{ config.id }}", "client_secret": "{{ config.secret }}",
				"scope": "{{ config.scope }}", "params": map[string]any{"soid": "{{ config.org }}"},
			}
			prefix := "Bearer"
			if grant == "refresh_token" {
				params["grant_type"], params["refresh_token"], params["header_prefix"] = grant, "{{ config.refresh }}", "Zoho-oauthtoken"
				prefix = "Zoho-oauthtoken"
			}
			creds := map[string]string{"id": "client", "secret": "secret", "scope": "read", "org": "org", "refresh": "refresh"}
			a, err := Build("oauth2_cc", params, creds)
			if err != nil {
				t.Fatal(err)
			}
			for i, want := range []string{"token-1", "token-1", "token-2"} {
				if i == 2 {
					a.(*oauth2CC).cache.Store(&cachedToken{token: "expired", expiresAt: time.Now().Add(-time.Minute)})
				}
				req := httptest.NewRequest("GET", "https://example.com/data", nil)
				if err := a.Apply(t.Context(), req, template.Scope{Config: creds}); err != nil {
					t.Fatal(err)
				}
				if got := req.Header.Get("Authorization"); got != prefix+" "+want {
					t.Fatalf("authorization = %q", got)
				}
			}
			if calls.Load() != 2 {
				t.Fatalf("token requests = %d", calls.Load())
			}
		})
	}
}
