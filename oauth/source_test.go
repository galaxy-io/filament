package oauth

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestRefreshRotationAcrossSources(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		id, secret, ok := r.BasicAuth()
		if !ok || id != url.QueryEscape("client:id") || secret != url.QueryEscape("secret +") || r.Form.Get("client_id") != "" || r.Form.Get("client_secret") != "" {
			t.Error("incorrect Basic client authentication")
		}
		n := calls.Add(1)
		want := "initial"
		if n > 1 {
			want = "{{ opaque.rotation }}"
		}
		if r.Form.Get("refresh_token") != want || r.Form.Get("grant_type") != RefreshToken {
			t.Error("incorrect refresh grant")
		}
		rotation := ""
		if n == 1 {
			rotation = `,"refresh_token":"{{ opaque.rotation }}"`
		}
		fmt.Fprintf(w, `{"access_token":"access-%d","expires_in":"3600"%s}`, n, rotation)
	}))
	defer api.Close()
	store := NewMemoryStore(Token{RefreshToken: "initial"})
	config := Config{TokenURL: api.URL, ClientID: "client:id", ClientSecret: "secret +", ClientAuth: ClientSecretBasic, GrantType: RefreshToken, HTTPClient: api.Client()}
	for round := 1; round <= 3; round++ {
		if round > 1 {
			_, err := store.Update(t.Context(), func(_ context.Context, token Token) (Token, error) {
				token.ExpiresAt = time.Now().Add(-time.Minute)
				return token, nil
			})
			if err != nil {
				t.Fatal(err)
			}
		}
		var wg sync.WaitGroup
		for range 20 {
			// Independent sources share authorization state, as separate
			// connectors would. The Store owns refresh serialization.
			source := mustSource(t, config, store)
			wg.Go(func() {
				token, err := source.Token(t.Context())
				if err != nil {
					t.Error(err)
					return
				}
				if token.AccessToken != fmt.Sprintf("access-%d", round) || token.RefreshToken != "{{ opaque.rotation }}" {
					t.Error("incorrect committed token")
				}
			})
		}
		wg.Wait()
		if got := calls.Load(); got != int32(round) {
			t.Fatalf("exchanges = %d, want %d", got, round)
		}
	}
}

func TestPersistenceFailureDoesNotExchangeAgain(t *testing.T) {
	for _, committed := range []bool{false, true} {
		t.Run(fmt.Sprint(committed), func(t *testing.T) {
			var calls atomic.Int32
			api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				fmt.Fprint(w, `{"access_token":"access","refresh_token":"rotated","expires_in":3600}`)
			}))
			defer api.Close()
			stored := Token{RefreshToken: "initial"}
			fail := true
			saveErr := errors.New("save failed")
			store := StoreFunc(func(ctx context.Context, fn func(context.Context, Token) (Token, error)) (Token, error) {
				next, err := fn(ctx, stored)
				if err != nil {
					return Token{}, err
				}
				if fail {
					fail = false
					if committed {
						stored = next
					}
					return Token{}, saveErr
				}
				stored = next
				return stored, nil
			})
			source := mustSource(t, refreshConfig(api), store)
			if token, err := source.Token(t.Context()); !errors.Is(err, saveErr) || token.AccessToken != "" {
				t.Fatal("uncommitted access token escaped")
			}
			if _, err := source.Token(t.Context()); err != nil {
				t.Fatal(err)
			}
			if calls.Load() != 1 || stored.RefreshToken != "rotated" {
				t.Fatal("successful exchange was lost or repeated")
			}
		})
	}
}

func TestPendingTokenDoesNotOverwriteReplacement(t *testing.T) {
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, `{"access_token":"old-access","refresh_token":"old-rotation","expires_in":3600}`)
	}))
	defer api.Close()
	stored := Token{RefreshToken: "initial"}
	fail := true
	store := StoreFunc(func(ctx context.Context, fn func(context.Context, Token) (Token, error)) (Token, error) {
		next, err := fn(ctx, stored)
		if err != nil {
			return Token{}, err
		}
		if fail {
			fail = false
			return Token{}, errors.New("save failed")
		}
		stored = next
		return stored, nil
	})
	source := mustSource(t, refreshConfig(api), store)
	if _, err := source.Token(t.Context()); err == nil {
		t.Fatal("expected save failure")
	}
	stored = Token{AccessToken: "new-authorization", RefreshToken: "new-refresh", ExpiresAt: time.Now().Add(time.Hour)}
	token, err := source.Token(t.Context())
	if err != nil || token.AccessToken != "new-authorization" {
		t.Fatal("pending response overwrote a newer authorization")
	}
}

func TestCanceledWaiterDoesNotCancelRefresh(t *testing.T) {
	started, release := make(chan struct{}), make(chan struct{})
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		close(started)
		<-release
		fmt.Fprint(w, `{"access_token":"access","expires_in":3600}`)
	}))
	defer api.Close()
	source := mustSource(t, refreshConfig(api), NewMemoryStore(Token{RefreshToken: "initial"}))
	done := make(chan error, 1)
	go func() { _, err := source.Token(t.Context()); done <- err }()
	<-started
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	_, err := source.Token(ctx)
	close(release)
	if !errors.Is(err, context.Canceled) {
		t.Error("waiter did not honor cancellation")
	}
	if err := <-done; err != nil {
		t.Fatal(err)
	}
}

func TestEndpointErrorsAreSafeAndNotRetried(t *testing.T) {
	for _, tc := range []struct {
		status int
		body   string
		reauth bool
	}{
		{400, `{"error":"invalid_grant","error_description":"private-credential"}`, true},
		{401, `{"error":"invalid_client"}`, false},
		{200, `{"error":"invalid_grant"}`, true},
		{500, `{"error":"private-credential"}`, false},
		{200, `{"access_token": "private-credential", "expires_in": "private-credential"}`, false},
		{200, `{"private-credential`, false},
	} {
		t.Run(fmt.Sprint(tc.status, tc.body), func(t *testing.T) {
			var calls atomic.Int32
			api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				calls.Add(1)
				w.WriteHeader(tc.status)
				fmt.Fprint(w, tc.body)
			}))
			defer api.Close()
			source := mustSource(t, refreshConfig(api), NewMemoryStore(Token{RefreshToken: "initial"}))
			_, err := source.Token(t.Context())
			if err == nil || strings.Contains(err.Error(), "private-credential") {
				t.Fatalf("unsafe or missing error: %v", err)
			}
			if errors.Is(err, ErrReauthorizationRequired) != tc.reauth || calls.Load() != 1 {
				t.Fatal("incorrect error classification or request retry")
			}
		})
	}
}

func TestExpiryValidation(t *testing.T) {
	for _, value := range []string{`0`, `-1`, `null`, `1.5`, `"oops"`, `9223372036854775807`} {
		if _, err := expiresIn([]byte(value)); err == nil {
			t.Errorf("accepted invalid expires_in %s", value)
		}
	}
	for _, value := range []string{`3600`, `"3600"`} {
		if ttl, err := expiresIn([]byte(value)); err != nil || ttl != time.Hour {
			t.Errorf("expiry %s: %v, %v", value, ttl, err)
		}
	}
}

func TestConfigValidation(t *testing.T) {
	for _, config := range []Config{
		{TokenURL: "http://example.com"},
		{TokenURL: "https://user:secret@example.com"},
		{TokenURL: "https://example.com/#fragment"},
		{TokenURL: "https://example.com", ClientID: "id"},
		{TokenURL: "https://example.com", ClientID: "id", ClientSecret: "secret", ClientAuth: "guess"},
		{TokenURL: "https://example.com", ClientID: "id", ClientSecret: "secret", GrantType: "authorization_code"},
		{TokenURL: "https://example.com", ClientID: "id", ClientSecret: "secret", Params: map[string]string{"grant_type": "override"}},
		{TokenURL: "https://example.com", ClientID: "id", ClientSecret: "secret", GrantType: RefreshToken},
	} {
		if _, err := NewSource(config, nil); err == nil {
			t.Error("accepted invalid config")
		}
	}
}

func TestStoreContextBoundsExchange(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
	}))
	defer api.Close()
	store := StoreFunc(func(ctx context.Context, fn func(context.Context, Token) (Token, error)) (Token, error) {
		owned, cancel := context.WithCancel(ctx)
		cancel()
		return fn(owned, Token{RefreshToken: "initial"})
	})
	source := mustSource(t, refreshConfig(api), store)
	if _, err := source.Token(t.Context()); !errors.Is(err, context.Canceled) {
		t.Fatalf("lost transaction context: %v", err)
	}
	if calls.Load() != 0 {
		t.Fatal("exchanged a token after losing storage ownership")
	}
}

func TestRefreshReusesAnotherCallersReplacement(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		calls.Add(1)
		fmt.Fprint(w, `{"access_token":"replacement","refresh_token":"rotated","expires_in":3600}`)
	}))
	defer api.Close()
	store := NewMemoryStore(Token{AccessToken: "rejected", RefreshToken: "initial", ExpiresAt: time.Now().Add(time.Hour)})
	first, second := mustSource(t, refreshConfig(api), store), mustSource(t, refreshConfig(api), store)
	for _, source := range []*Source{first, second} {
		token, err := source.Refresh(t.Context(), "rejected")
		if err != nil || token.AccessToken != "replacement" {
			t.Fatalf("refresh failed: %v", err)
		}
	}
	if calls.Load() != 1 {
		t.Fatal("a stale rejection rotated the new authorization again")
	}
	if _, err := first.Refresh(t.Context(), ""); err == nil {
		t.Fatal("unconditional refresh was accepted")
	}
}

func TestExpiredPendingTokenIsSavedBeforeNextExchange(t *testing.T) {
	var calls atomic.Int32
	api := httptest.NewTLSServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if err := r.ParseForm(); err != nil {
			t.Error(err)
		}
		n := calls.Add(1)
		want := "initial"
		if n > 1 {
			want = "rotated"
		}
		if r.Form.Get("refresh_token") != want {
			t.Error("expired pending access token lost its refresh token")
		}
		fmt.Fprint(w, `{"access_token":"access","refresh_token":"rotated","expires_in":3600}`)
	}))
	defer api.Close()
	stored := Token{RefreshToken: "initial"}
	fail := true
	store := StoreFunc(func(ctx context.Context, fn func(context.Context, Token) (Token, error)) (Token, error) {
		next, err := fn(ctx, stored)
		if err != nil {
			return Token{}, err
		}
		if fail {
			fail = false
			return Token{}, errors.New("save failed")
		}
		stored = next
		return next, nil
	})
	source := mustSource(t, refreshConfig(api), store)
	now := time.Now()
	source.now = func() time.Time { return now }
	if _, err := source.Token(t.Context()); err == nil {
		t.Fatal("expected save failure")
	}
	now = now.Add(2 * time.Hour)
	token, err := source.Token(t.Context())
	if err != nil || !token.ExpiresAt.After(now) || calls.Load() != 2 {
		t.Fatal("did not recover from an expired pending exchange")
	}
}

func refreshConfig(api *httptest.Server) Config {
	return Config{TokenURL: api.URL, ClientID: "client", ClientSecret: "secret", GrantType: RefreshToken, HTTPClient: api.Client()}
}

func mustSource(t *testing.T, config Config, store Store) *Source {
	t.Helper()
	source, err := NewSource(config, store)
	if err != nil {
		t.Fatal(err)
	}
	return source
}
