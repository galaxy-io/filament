package httpapi

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/http/auth"
	"github.com/galaxy-io/filament/oauth"
)

func TestSourceBindsOAuthStoreForConnectionProbe(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer stored-access" {
			t.Error("source did not use the bound token store")
		}
		fmt.Fprint(w, `[]`)
	}))
	defer api.Close()
	source := NewManifest([]byte(fmt.Sprintf(`
version: 1
name: oauth-fixture
display_name: OAuth Fixture
description: Shared token storage fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: %s
  auth:
    oauth2:
      token_url: https://unused.example.com/token
      client_id: id
      client_secret: secret
      grant_type: refresh_token
      refresh_token: stale-configured-token
resources:
  - name: items
    path: /items
`, api.URL)))
	source.SetOAuthStore(oauth.NewMemoryStore(oauth.Token{
		AccessToken: "stored-access", RefreshToken: "stored-refresh", ExpiresAt: time.Now().Add(time.Hour),
	}))
	config := filament.NewConfig(nil)
	if err := source.Configure(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = source.Teardown(t.Context()) }()
	if err := source.connector.TestConnection(t.Context()); err != nil {
		t.Fatal(err)
	}
	// TestConnection constructs a fresh underlying connector. It must retain
	// the same store instead of falling back to the configured refresh token.
	if err := source.TestConnection(t.Context(), config); err != nil {
		t.Fatal(err)
	}
	if calls.Load() != 2 {
		t.Fatalf("probes = %d", calls.Load())
	}
}

const managedOAuthManifest = `
version: 1
name: managed
display_name: Managed
description: Managed refresh fixture.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
config:
  account: {type: string}
  client_id: {type: string, default: ''}
  client_secret: {type: secret, default: ''}
  refresh_token: {type: secret, default: ''}
  access_token: {type: secret, default: ''}
connection:
  base_url: BASE
  auth:
    oauth2:
      token_url: BASE/token
      client_id: config.client_id
      client_secret: config.client_secret
      client_auth: client_secret_basic
      grant_type: refresh_token
      refresh_token: config.refresh_token
      fallback_token: config.access_token
      managed_refresh: true
      binding: [managed, '{{ config.account }}', '{{ config.client_id }}', '{{ config.client_secret }}']
resources:
  - name: items
    path: /items
    response: {records: $, cardinality: one}
`

func TestManagedOAuthBindingAndRejectedToken(t *testing.T) {
	var tokenCalls, reads atomic.Int32
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			tokenCalls.Add(1)
			id, secret, ok := r.BasicAuth()
			if !ok || id != "client" || secret != "secret" {
				t.Error("missing client Basic authentication")
			}
			if err := r.ParseForm(); err != nil {
				t.Error(err)
			}
			if r.Form.Get("refresh_token") != "stored-refresh" {
				t.Error("did not use saved refresh token")
			}
			fmt.Fprint(w, `{"access_token":"replacement","refresh_token":"rotated","expires_in":3600}`)
			return
		}
		reads.Add(1)
		if r.Header.Get("Authorization") != "Bearer replacement" {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		fmt.Fprint(w, `{}`)
	}))
	defer api.Close()
	src := NewManifest([]byte(strings.ReplaceAll(managedOAuthManifest, "BASE", api.URL)))
	values := map[string]any{"account": "company", "client_id": "client", "client_secret": "secret", "refresh_token": "initial"}
	cfg := filament.NewConfig(values)
	field, binding := src.OAuthRefreshConfig(cfg)
	if field != "refresh_token" || binding == "" {
		t.Fatalf("binding=%s %s", field, binding)
	}
	values["refresh_token"] = "rotated"
	if _, next := src.OAuthRefreshConfig(filament.NewConfig(values)); next != binding {
		t.Fatal("rotation changed binding")
	}
	values["account"] = "other"
	if _, next := src.OAuthRefreshConfig(filament.NewConfig(values)); next == binding {
		t.Fatal("account change retained binding")
	}
	values["account"] = "company"
	// Validation never exchanges a managed rotating token without a store.
	if err := src.TestConnection(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 0 || reads.Load() != 0 {
		t.Fatal("validation called upstream")
	}
	if err := src.Configure(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	if err := src.connector.TestConnection(t.Context()); err == nil {
		t.Fatal("unbound managed refresh accepted")
	}
	store := oauth.NewMemoryStore(oauth.Token{AccessToken: "rejected", RefreshToken: "stored-refresh", ExpiresAt: time.Now().Add(time.Hour)})
	src.SetOAuthStore(store)
	if err := src.Configure(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	src.connector.client = api.Client()
	auth.WithOAuthHTTPClient(src.connector.builder.Auth, api.Client())
	if err := src.connector.TestConnection(t.Context()); err != nil {
		t.Fatal(err)
	}
	if tokenCalls.Load() != 1 || reads.Load() != 2 {
		t.Fatalf("tokens=%d reads=%d", tokenCalls.Load(), reads.Load())
	}
	token, err := store.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) { return token, nil })
	if err != nil || token.RefreshToken != "rotated" {
		t.Fatalf("token rotation not persisted: %v", err)
	}
}

func TestManagedOAuthAccessTokenFallback(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		if r.Header.Get("Authorization") != "Bearer manual" {
			t.Error("incorrect fallback token")
		}
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer api.Close()
	src := NewManifest([]byte(strings.ReplaceAll(managedOAuthManifest, "BASE", api.URL)))
	cfg := filament.NewConfig(map[string]any{"access_token": "manual"})
	if err := src.TestConnection(t.Context(), cfg); err == nil {
		t.Fatal("401 accepted")
	}
	if calls.Load() != 1 {
		t.Fatal("static token retried or exchanged")
	}
	if field, _ := src.OAuthRefreshConfig(cfg); field != "" {
		t.Fatal("static token requested managed store")
	}
	for _, values := range []map[string]any{{}, {"client_id": "x"}, {"refresh_token": "x"}, {"access_token": "manual", "client_secret": "x"}} {
		if err := src.Validate(filament.NewConfig(values)); err == nil {
			t.Fatal("incomplete credentials accepted")
		}
	}
}

func TestManagedOAuthRejectsInvalidDeclarations(t *testing.T) {
	for name, declaration := range map[string]string{
		"wrong grant":           strings.Replace(managedOAuthManifest, "grant_type: refresh_token", "grant_type: client_credentials", 1),
		"nonsecret":             strings.Replace(managedOAuthManifest, "refresh_token: {type: secret", "refresh_token: {type: string", 1),
		"indirect refresh":      strings.Replace(managedOAuthManifest, "refresh_token: config.refresh_token", "refresh_token: '{{ config.refresh_token }}suffix'", 1),
		"missing binding":       strings.Replace(managedOAuthManifest, "      binding: [managed, '{{ config.account }}', '{{ config.client_id }}', '{{ config.client_secret }}']\n", "", 1),
		"invalid binding scope": strings.Replace(managedOAuthManifest, "'{{ config.account }}'", "'{{ parent.account }}'", 1),
	} {
		t.Run(name, func(t *testing.T) {
			src := NewManifest([]byte(strings.ReplaceAll(declaration, "BASE", "https://example.com")))
			if src.manifestErr == nil {
				t.Fatal("invalid managed auth accepted")
			}
		})
	}
}

func TestManagedOAuthSecondUnauthorizedStops(t *testing.T) {
	var tokenCalls, reads atomic.Int32
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/token" {
			tokenCalls.Add(1)
			fmt.Fprint(w, `{"access_token":"replacement","refresh_token":"rotated","expires_in":3600}`)
			return
		}
		reads.Add(1)
		w.WriteHeader(http.StatusUnauthorized)
	}))
	defer api.Close()
	src := NewManifest([]byte(strings.ReplaceAll(managedOAuthManifest, "BASE", api.URL)))
	src.SetOAuthStore(oauth.NewMemoryStore(oauth.Token{AccessToken: "rejected", RefreshToken: "saved", ExpiresAt: time.Now().Add(time.Hour)}))
	cfg := filament.NewConfig(map[string]any{"client_id": "client", "client_secret": "secret", "refresh_token": "initial"})
	if err := src.Configure(t.Context(), cfg); err != nil {
		t.Fatal(err)
	}
	src.connector.client = api.Client()
	auth.WithOAuthHTTPClient(src.connector.builder.Auth, api.Client())
	if err := src.connector.TestConnection(t.Context()); err == nil {
		t.Fatal("second 401 accepted")
	}
	if tokenCalls.Load() != 1 || reads.Load() != 2 {
		t.Fatalf("tokens=%d reads=%d", tokenCalls.Load(), reads.Load())
	}
}
