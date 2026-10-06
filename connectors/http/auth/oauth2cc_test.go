package auth

import (
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/template"
)

func TestOAuthGrantsCacheAndRenewal(t *testing.T) {
	for _, grant := range []string{"client_credentials", "refresh_token"} {
		t.Run(grant, func(t *testing.T) {
			var calls atomic.Int32
			api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			a.(*oauth2CC).httpClient = api.Client()
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

func TestOAuthRefreshTokenRotation(t *testing.T) {
	// The third response omits rotation; the fourth must retain the second
	// replacement literally, even though it resembles a template.
	tokens := []string{"original", "replacement", "{{ config.not_a_template }}", "{{ config.not_a_template }}"}
	var calls atomic.Int32
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(calls.Add(1)) - 1
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		if i >= len(tokens) || r.Form.Get("refresh_token") != tokens[i] {
			t.Error("unexpected token reuse or extra renewal")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		extra := ""
		if i < 2 {
			extra = fmt.Sprintf(",\"refresh_token\":%q", tokens[i+1])
		}
		fmt.Fprintf(w, `{"access_token":"access","expires_in":3600%s}`, extra)
	}))
	t.Cleanup(api.Close)
	a, err := newOAuth2CC(map[string]any{"token_url": api.URL, "client_id": "id", "client_secret": "secret", "grant_type": "refresh_token", "refresh_token": "original"})
	if err != nil {
		t.Fatal(err)
	}
	oc := a.(*oauth2CC)
	oc.httpClient = api.Client()
	for range tokens {
		oc.cache.Store(nil)
		var wg sync.WaitGroup
		for range 20 {
			wg.Go(func() {
				req := httptest.NewRequest("GET", "https://api.example.com", nil)
				if err := oc.Apply(t.Context(), req, template.Scope{}); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
	}
	if calls.Load() != 4 {
		t.Fatalf("renewals = %d", calls.Load())
	}
}

func TestOAuthRejectsUnsafeRenderedURLs(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { calls.Add(1) }))
	t.Cleanup(api.Close)
	for _, location := range []string{api.URL, "https://user:secret@example.com/token", "https://example.com/token#fragment", "/token", "https:///token", "https://%zz"} {
		a, err := newOAuth2CC(map[string]any{"token_url": "{{ config.endpoint }}", "client_id": "id", "client_secret": "secret"})
		if err != nil {
			t.Fatal(err)
		}
		err = a.Apply(t.Context(), httptest.NewRequest("GET", "https://api.example.com", nil), template.Scope{Config: map[string]string{"endpoint": location}})
		if !errors.Is(err, errs.ErrAuthRefresh) {
			t.Fatalf("unsafe URL accepted: %v", err)
		}
	}
	if calls.Load() != 0 {
		t.Fatal("credentials sent over HTTP")
	}
}

func TestOAuthRejectsRedirectsAndNonSuccess(t *testing.T) {
	var forwarded atomic.Int32
	target := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) { forwarded.Add(1) }))
	t.Cleanup(target.Close)
	for _, status := range []int{301, 302, 303, 307, 308, 400, 500} {
		t.Run(fmt.Sprint(status), func(t *testing.T) {
			api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Location", target.URL)
				w.WriteHeader(status)
				fmt.Fprint(w, `{"access_token":"must-not-be-accepted"}`)
			}))
			t.Cleanup(api.Close)
			a, err := newOAuth2CC(map[string]any{"token_url": api.URL, "client_id": "id", "client_secret": "secret"})
			if err != nil {
				t.Fatal(err)
			}
			a.(*oauth2CC).httpClient = api.Client()
			if err := a.Apply(t.Context(), httptest.NewRequest("GET", "https://api.example.com", nil), template.Scope{}); !errors.Is(err, errs.ErrAuthRefresh) {
				t.Fatalf("status %d: %v", status, err)
			}
		})
	}
	if forwarded.Load() != 0 {
		t.Fatal("credentials forwarded to redirect target")
	}
}
