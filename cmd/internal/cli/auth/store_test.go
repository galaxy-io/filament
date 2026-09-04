package auth

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func testStore(t *testing.T) Store {
	t.Helper()
	return Store{Path: filepath.Join(t.TempDir(), "credentials.yaml")}
}

func testProfile() Profile {
	return Profile{
		Issuer:       "https://auth.example.com",
		ClientID:     "331693296033546241",
		ClientSecret: "s3cr3t",
		Scopes:       []string{"openid", "urn:zitadel:iam:user:resourceowner"},
	}
}

func TestStoreMissingFileIsEmpty(t *testing.T) {
	doc, err := testStore(t).Load()
	if err != nil {
		t.Fatalf("load: %v", err)
	}
	if len(doc.Profiles) != 0 {
		t.Fatalf("expected empty document, got %d profiles", len(doc.Profiles))
	}
}

func TestStoreRoundTrip(t *testing.T) {
	store := testStore(t)
	if err := store.Put("prod", testProfile()); err != nil {
		t.Fatalf("put: %v", err)
	}
	got, err := store.Get("prod")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.ClientID != testProfile().ClientID || len(got.Scopes) != 2 {
		t.Fatalf("unexpected profile %+v", got)
	}
	info, err := os.Stat(store.Path)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	if perm := info.Mode().Perm(); perm != 0o600 {
		t.Fatalf("credentials file mode %o, want 600", perm)
	}
}

func TestStorePutValidates(t *testing.T) {
	store := testStore(t)
	if err := store.Put("", testProfile()); err == nil {
		t.Fatal("expected error for empty name")
	}
	profile := testProfile()
	profile.ClientSecret = ""
	if err := store.Put("prod", profile); err == nil {
		t.Fatal("expected error for missing secret")
	}
}

func TestStoreGetUnknownProfile(t *testing.T) {
	if _, err := testStore(t).Get("nope"); err == nil {
		t.Fatal("expected error for unknown profile")
	}
}

func TestStoreDelete(t *testing.T) {
	store := testStore(t)
	if err := store.Put("prod", testProfile()); err != nil {
		t.Fatalf("put: %v", err)
	}
	if err := store.Delete("prod"); err != nil {
		t.Fatalf("delete: %v", err)
	}
	if _, err := store.Get("prod"); err == nil {
		t.Fatal("expected profile to be gone")
	}
	if err := store.Delete("prod"); err != nil {
		t.Fatalf("deleting absent profile: %v", err)
	}
}

func TestStoreSaveCache(t *testing.T) {
	store := testStore(t)
	if err := store.Put("prod", testProfile()); err != nil {
		t.Fatalf("put: %v", err)
	}
	expires := time.Now().Add(time.Hour).UTC().Truncate(time.Second)
	if err := store.SaveCache("prod", Cache{AccessToken: "tok", ExpiresAt: expires}); err != nil {
		t.Fatalf("save cache: %v", err)
	}
	got, err := store.Get("prod")
	if err != nil {
		t.Fatalf("get: %v", err)
	}
	if got.Cache == nil || got.Cache.AccessToken != "tok" || !got.Cache.ExpiresAt.Equal(expires) {
		t.Fatalf("unexpected cache %+v", got.Cache)
	}
	if err := store.SaveCache("nope", Cache{}); err == nil {
		t.Fatal("expected error caching against unknown profile")
	}
}
