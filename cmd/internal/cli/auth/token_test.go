package auth

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

// fakeIssuer serves OIDC discovery and a client-credentials token endpoint,
// recording how it was called.
type fakeIssuer struct {
	server      *httptest.Server
	tokenCalls  int
	lastGrant   string
	lastScope   string
	lastID      string
	lastSecret  string
	accessToken string
}

func newFakeIssuer(t *testing.T) *fakeIssuer {
	t.Helper()
	issuer := &fakeIssuer{accessToken: "minted-token"}
	mux := http.NewServeMux()
	mux.HandleFunc("/.well-known/openid-configuration", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(map[string]string{
			"token_endpoint": issuer.server.URL + "/oauth/v2/token",
		})
	})
	mux.HandleFunc("/oauth/v2/token", func(w http.ResponseWriter, r *http.Request) {
		issuer.tokenCalls++
		issuer.lastID, issuer.lastSecret, _ = r.BasicAuth()
		_ = r.ParseForm()
		issuer.lastGrant = r.PostForm.Get("grant_type")
		issuer.lastScope = r.PostForm.Get("scope")
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprintf(w, `{"access_token":%q,"token_type":"Bearer","expires_in":3600}`, issuer.accessToken)
	})
	issuer.server = httptest.NewServer(mux)
	t.Cleanup(issuer.server.Close)
	return issuer
}

func testSource(t *testing.T, issuer *fakeIssuer) Source {
	t.Helper()
	store := testStore(t)
	profile := testProfile()
	profile.Issuer = issuer.server.URL
	if err := store.Put("prod", profile); err != nil {
		t.Fatalf("put: %v", err)
	}
	return Source{Store: store, Profile: "prod", Client: issuer.server.Client()}
}

func TestSourceMintsAndPersists(t *testing.T) {
	issuer := newFakeIssuer(t)
	source := testSource(t, issuer)
	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("token: %v", err)
	}
	if token != "minted-token" {
		t.Fatalf("token %q, want minted-token", token)
	}
	if issuer.lastGrant != "client_credentials" {
		t.Fatalf("grant %q, want client_credentials", issuer.lastGrant)
	}
	if issuer.lastID != testProfile().ClientID || issuer.lastSecret != testProfile().ClientSecret {
		t.Fatalf("basic auth %q/%q", issuer.lastID, issuer.lastSecret)
	}
	if issuer.lastScope != "openid urn:zitadel:iam:user:resourceowner" {
		t.Fatalf("scope %q", issuer.lastScope)
	}
	profile, err := source.Store.Get("prod")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if profile.Cache == nil || profile.Cache.AccessToken != "minted-token" {
		t.Fatalf("cache not persisted: %+v", profile.Cache)
	}
	if remaining := time.Until(profile.Cache.ExpiresAt); remaining < 55*time.Minute {
		t.Fatalf("cached expiry only %s away", remaining)
	}
}

func TestSourceReusesCachedToken(t *testing.T) {
	issuer := newFakeIssuer(t)
	source := testSource(t, issuer)
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatalf("first token: %v", err)
	}
	issuer.accessToken = "should-not-be-minted"
	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("second token: %v", err)
	}
	if token != "minted-token" {
		t.Fatalf("token %q, want cached minted-token", token)
	}
	if issuer.tokenCalls != 1 {
		t.Fatalf("token endpoint called %d times, want 1", issuer.tokenCalls)
	}
}

func TestSourceRefreshesNearExpiry(t *testing.T) {
	issuer := newFakeIssuer(t)
	source := testSource(t, issuer)
	if _, err := source.Token(context.Background()); err != nil {
		t.Fatalf("first token: %v", err)
	}
	issuer.accessToken = "second-token"
	source.Now = func() time.Time { return time.Now().Add(time.Hour) }
	token, err := source.Token(context.Background())
	if err != nil {
		t.Fatalf("second token: %v", err)
	}
	if token != "second-token" {
		t.Fatalf("token %q, want second-token", token)
	}
	if issuer.tokenCalls != 2 {
		t.Fatalf("token endpoint called %d times, want 2", issuer.tokenCalls)
	}
}

func TestSourceUnknownProfile(t *testing.T) {
	source := Source{Store: testStore(t), Profile: "nope"}
	if _, err := source.Token(context.Background()); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestTokenEndpointErrors(t *testing.T) {
	server := httptest.NewServer(http.NotFoundHandler())
	t.Cleanup(server.Close)
	if _, err := tokenEndpoint(context.Background(), server.Client(), server.URL); err == nil {
		t.Fatal("expected error for missing discovery document")
	}
}
