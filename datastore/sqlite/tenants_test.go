package sqlite

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
)

func TestTenantExternalID(t *testing.T) {
	ctx := context.Background()
	store := NewMemory()
	id, err := store.ResolveTenant(ctx, "org-1", "Org One")
	if err != nil {
		t.Fatal(err)
	}
	if ext, err := store.TenantExternalID(ctx, id); err != nil || ext != "org-1" {
		t.Fatalf("TenantExternalID = %q, %v", ext, err)
	}
	if err := store.EnsureTenant(ctx, "local", "Local"); err != nil {
		t.Fatal(err)
	}
	if ext, err := store.TenantExternalID(ctx, "local"); err != nil || ext != "" {
		t.Fatalf("provider-less tenant = %q, %v", ext, err)
	}
	if _, err := store.TenantExternalID(ctx, "missing"); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("missing tenant: %v", err)
	}
}
