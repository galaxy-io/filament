package postgres_test

import (
	"context"
	"os"
	"testing"

	"github.com/galaxy-io/filament"
	datastorepostgres "github.com/galaxy-io/filament/datastore/postgres"
	secretpostgres "github.com/galaxy-io/filament/secret/postgres"
)

func TestProviderRoundTrip(t *testing.T) {
	const tenantID = "11111111-1111-4111-8111-111111111111"
	dsn := os.Getenv("FILAMENT_TEST_POSTGRES_DSN")
	if dsn == "" {
		t.Skip("FILAMENT_TEST_POSTGRES_DSN not set")
	}
	ctx := context.Background()
	db, err := datastorepostgres.NewSQLDB(dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()
	if err := datastorepostgres.Migrate(db); err != nil {
		t.Fatal(err)
	}
	pool, err := datastorepostgres.NewPool(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if _, err := pool.Exec(ctx, "DELETE FROM secrets"); err != nil {
		t.Fatal(err)
	}
	store := datastorepostgres.New(pool)
	if err := store.EnsureTenant(ctx, tenantID, "Tenant A"); err != nil {
		t.Fatal(err)
	}
	p, err := secretpostgres.New(pool, "test-key-v1", make([]byte, 32))
	if err != nil {
		t.Fatal(err)
	}
	ref := "tenant-a/pg-dsn"
	original := filament.Secret{Tenant: tenantID, Value: []byte("postgres://user:pw@host/db"), Meta: map[string]string{"rotated": "2026-01-01"}}
	if err := p.Write(ctx, ref, original); err != nil {
		t.Fatal(err)
	}
	got, err := p.Read(ctx, ref)
	if err != nil {
		t.Fatal(err)
	}
	if string(got.Value) != string(original.Value) || got.Meta["rotated"] != "2026-01-01" {
		t.Fatalf("round trip = %+v", got)
	}
	if err := p.Delete(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if _, err := p.Read(ctx, ref); err == nil {
		t.Fatal("expected not found after delete")
	}
}
