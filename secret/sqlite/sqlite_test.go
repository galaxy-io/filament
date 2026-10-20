package sqlite_test

import (
	"bytes"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	datastoresqlite "github.com/galaxy-io/filament/datastore/sqlite"
	secretsqlite "github.com/galaxy-io/filament/secret/sqlite"
)

func TestRotationUpdatesExistingTenantSecret(t *testing.T) {
	db := datastoresqlite.NewMemory()
	defer func() { _ = db.Close() }()
	p, err := secretsqlite.New(db.DB(), "test", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ref := "shared-reference"
	value := filament.Secret{Tenant: filament.DefaultTenantID, Value: []byte("initial"), Meta: map[string]string{"field": "refresh_token"}}
	if err := p.Write(t.Context(), ref, value); err != nil {
		t.Fatal(err)
	}
	var before int64
	if err := db.DB().QueryRowContext(t.Context(), "SELECT created_at FROM secrets WHERE ref = ?", ref).Scan(&before); err != nil {
		t.Fatal(err)
	}
	value.Value = []byte("rotated-refresh")
	if err := p.RotateSecret(t.Context(), ref, value); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read(t.Context(), ref)
	if err != nil || string(got.Value) != "rotated-refresh" || got.Meta["field"] != "refresh_token" {
		t.Fatal("rotation not persisted")
	}
	var count int
	var created int64
	var encrypted []byte
	if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*), created_at, ciphertext FROM secrets WHERE ref = ?", ref).Scan(&count, &created, &encrypted); err != nil {
		t.Fatal(err)
	}
	if count != 1 || created != before || bytes.Contains(encrypted, value.Value) {
		t.Fatal("rotation replaced row or stored plaintext")
	}
	wrong := value
	wrong.Tenant = "other-tenant"
	if err := p.RotateSecret(t.Context(), ref, wrong); !errors.Is(err, filament.ErrNotFound) {
		t.Fatal("rotation crossed tenant boundary")
	}
	if err := p.Delete(t.Context(), ref); err != nil {
		t.Fatal(err)
	}
	if err := p.RotateSecret(t.Context(), ref, value); !errors.Is(err, filament.ErrNotFound) {
		t.Fatal("rotation recreated deleted secret")
	}
	var coordination int
	if err := db.DB().QueryRowContext(t.Context(), "SELECT count(*) FROM sqlite_master WHERE type='table' AND name='secret_coordination'").Scan(&coordination); err != nil || coordination != 0 {
		t.Fatal("coordination table remains")
	}
}
