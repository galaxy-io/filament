package secrets_test

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
	datastorepostgres "github.com/galaxy-io/filament/datastore/postgres"
	datastoresqlite "github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/oauth"
	oauthsecrets "github.com/galaxy-io/filament/oauth/secrets"
	secretenv "github.com/galaxy-io/filament/secret/env"
	secretpostgres "github.com/galaxy-io/filament/secret/postgres"
	secretsqlite "github.com/galaxy-io/filament/secret/sqlite"
)

const tenant = filament.TenantID("11111111-1111-4111-8111-111111111111")

func TestSQLiteDurableRefresh(t *testing.T) {
	path := filepath.Join(t.TempDir(), "oauth.db")
	open := func() (*datastoresqlite.Store, filament.Secrets) {
		db, err := datastoresqlite.Open(path)
		if err != nil {
			t.Fatal(err)
		}
		if err := db.EnsureTenant(t.Context(), tenant, "OAuth test"); err != nil {
			t.Fatal(err)
		}
		provider, err := secretsqlite.New(db.DB(), "test", make([]byte, 32))
		if err != nil {
			t.Fatal(err)
		}
		return db, provider
	}
	db, provider := open()
	defer func() { _ = db.Close() }()
	otherDB, otherProvider := open()
	defer func() { _ = otherDB.Close() }()
	testCoordinatedStore(t, provider, otherProvider)

	store := mustStore(t, provider, "restart")
	seed := oauth.Token{AccessToken: "private-access", RefreshToken: "private-refresh", ExpiresAt: time.Now().Add(time.Hour).UTC()}
	if err := store.Set(t.Context(), seed); err != nil {
		t.Fatal(err)
	}
	var ciphertext []byte
	if err := db.DB().QueryRowContext(t.Context(), "SELECT ciphertext FROM secrets WHERE ref = ?", "filament/"+string(tenant)+"/oauth/restart").Scan(&ciphertext); err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(ciphertext, []byte("private-")) {
		t.Fatal("plaintext token stored in database")
	}
	if err := db.Close(); err != nil {
		t.Fatal(err)
	}
	db, provider = open()
	store = mustStore(t, provider, "restart")
	token, err := store.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) { return token, nil })
	if err != nil || token != seed {
		t.Fatal("token did not survive database reopen")
	}
}

func TestPostgresDurableRefresh(t *testing.T) {
	dsn := os.Getenv("FILAMENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("FILAMENT_TEST_POSTGRES_DSN not set")
	}
	db, err := datastorepostgres.NewSQLDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := datastorepostgres.Migrate(db); err != nil {
		t.Fatal(err)
	}
	pool, err := datastorepostgres.NewPool(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := datastorepostgres.New(pool).EnsureTenant(t.Context(), tenant, "OAuth test"); err != nil {
		t.Fatal(err)
	}
	provider, err := secretpostgres.New(pool, "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	otherPool, err := datastorepostgres.NewPool(t.Context(), dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer otherPool.Close()
	other, err := secretpostgres.New(otherPool, "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	testCoordinatedStore(t, provider, other)
}

func testCoordinatedStore(t *testing.T, provider, other filament.Secrets) {
	t.Helper()
	id := uuid.NewString()
	store := mustStore(t, provider, id)
	second := mustStore(t, other, id)
	defer func() { _ = store.Delete(context.Background()) }()
	if err := store.Set(t.Context(), oauth.Token{RefreshToken: "initial"}); err != nil {
		t.Fatal(err)
	}
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
			t.Error("stale refresh token used")
		}
		fmt.Fprint(w, `{"access_token":"access","refresh_token":"rotated","expires_in":3600}`)
	}))
	defer api.Close()
	config := oauth.Config{TokenURL: api.URL, ClientID: "id", ClientSecret: "secret", GrantType: oauth.RefreshToken, HTTPClient: api.Client()}
	for round := 1; round <= 2; round++ {
		var wg sync.WaitGroup
		for i := range 10 {
			var bound oauth.Store = store
			if i%2 != 0 {
				bound = second
			}
			source, err := oauth.NewSource(config, bound)
			if err != nil {
				t.Fatal(err)
			}
			wg.Go(func() {
				if _, err := source.Token(t.Context()); err != nil {
					t.Error(err)
				}
			})
		}
		wg.Wait()
		if calls.Load() != int32(round) {
			t.Fatal("independent database clients raced token refresh")
		}
		if _, err := store.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) {
			token.ExpiresAt = time.Now().Add(-time.Hour)
			return token, nil
		}); err != nil {
			t.Fatal(err)
		}
	}
	rejected := errors.New("reject update")
	if _, err := store.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) {
		token.RefreshToken = "must-rollback"
		return token, rejected
	}); !errors.Is(err, rejected) {
		t.Fatal("callback error not returned")
	}
	if _, err := store.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) {
		if token.RefreshToken != "rotated" {
			t.Error("failed callback changed stored credentials")
		}
		return token, nil
	}); err != nil {
		t.Fatal(err)
	}
	otherTenant, err := oauthsecrets.New(provider, "22222222-2222-4222-8222-222222222222", id)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := otherTenant.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) {
		t.Error("another tenant read credential")
		return token, nil
	}); !errors.Is(err, filament.ErrNotFound) {
		t.Fatal("missing tenant authorization was accepted")
	}
	if err := store.Delete(t.Context()); err != nil {
		t.Fatal(err)
	}
	if _, err := second.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) {
		t.Error("deleted credential was read")
		return token, nil
	}); !errors.Is(err, filament.ErrNotFound) {
		t.Fatal("deleted credential was recreated")
	}
}

func TestUnsupportedProviderAndInvalidIdentity(t *testing.T) {
	if _, err := oauthsecrets.New(secretenv.New(), tenant, "credential"); err == nil {
		t.Fatal("read-only provider accepted")
	}
	if _, err := oauthsecrets.New(nil, tenant, "../other"); err == nil {
		t.Fatal("unsafe credential ID accepted")
	}
}

func mustStore(t *testing.T, provider filament.Secrets, id string) *oauthsecrets.Store {
	t.Helper()
	store, err := oauthsecrets.New(provider, tenant, id)
	if err != nil {
		t.Fatal(err)
	}
	return store
}
