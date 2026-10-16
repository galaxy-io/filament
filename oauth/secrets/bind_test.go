package secrets_test

import (
	"context"
	"errors"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	datastoresqlite "github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/oauth"
	oauthsecrets "github.com/galaxy-io/filament/oauth/secrets"
	secretenv "github.com/galaxy-io/filament/secret/env"
	secretsqlite "github.com/galaxy-io/filament/secret/sqlite"
)

func TestManagedRefreshSecretLifecycle(t *testing.T) {
	db := datastoresqlite.NewMemory()
	defer func() { _ = db.Close() }()
	provider, err := secretsqlite.New(db.DB(), "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ref := filament.ConnectionSecretRef(string(tenant), "connection", "refresh_token", 1)
	// Even JSON-looking refresh tokens must remain opaque without the marker.
	initial := `{"opaque":"initial-refresh"}`
	if err := provider.Write(t.Context(), ref, filament.Secret{Tenant: tenant, Value: []byte(initial), Meta: map[string]string{"field": "refresh_token"}}); err != nil {
		t.Fatal(err)
	}
	store, err := oauthsecrets.FromRefreshToken(provider, tenant, ref, "account-client-binding")
	if err != nil {
		t.Fatal(err)
	}
	rotated := oauth.Token{AccessToken: "access", RefreshToken: "rotated", ExpiresAt: time.Now().Add(time.Hour).UTC()}
	if _, err := store.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) {
		if token.RefreshToken != initial {
			t.Error("initial refresh token was parsed or changed")
		}
		return rotated, nil
	}); err != nil {
		t.Fatal(err)
	}
	saved, err := provider.Read(t.Context(), ref)
	if err != nil || saved.Meta["field"] != "refresh_token" || saved.Meta["oauth_format"] != "v1" {
		t.Fatal("secret metadata or format marker lost")
	}
	if strings.Contains(saved.Meta["oauth_format"], "rotated") {
		t.Fatal("token material stored in metadata")
	}
	reopened, err := oauthsecrets.FromRefreshToken(provider, tenant, ref, "account-client-binding")
	if err != nil {
		t.Fatal(err)
	}
	if token, err := reopened.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) { return token, nil }); err != nil || token != rotated {
		t.Fatal("rotation was not reused")
	}
	changed, err := oauthsecrets.FromRefreshToken(provider, tenant, ref, "different-account")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := changed.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) {
		t.Error("mismatched authorization reached token exchange")
		return token, nil
	}); err == nil {
		t.Fatal("changed authorization accepted")
	}
	if err := provider.Delete(t.Context(), ref); err != nil {
		t.Fatal(err)
	}
	if _, err := reopened.Update(t.Context(), func(_ context.Context, token oauth.Token) (oauth.Token, error) { return token, nil }); !errors.Is(err, filament.ErrNotFound) {
		t.Fatal("deleted connection credentials were resurrected")
	}
}

func TestManagedRefreshRejectsUnsafeReferences(t *testing.T) {
	for _, ref := range []string{"", "env:TOKEN", "filament/another-tenant/connection/token/v1"} {
		if _, err := oauthsecrets.FromRefreshToken(nil, tenant, ref, "binding"); err == nil {
			t.Error("unmanaged or cross-tenant reference accepted")
		}
	}
	ref := filament.ConnectionSecretRef(string(tenant), "connection", "refresh_token", 1)
	if _, err := oauthsecrets.FromRefreshToken(secretenv.New(), tenant, ref, "binding"); err == nil {
		t.Fatal("read-only provider accepted")
	}
}
