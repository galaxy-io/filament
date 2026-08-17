package runs

import (
	"testing"

	"github.com/google/uuid"

	"github.com/galaxy-io/filament"
)

func TestIDForReturnsUUID(t *testing.T) {
	req := filament.RunRequest{Tenant: filament.DefaultTenantID, IdempotencyKey: "client-token"}

	first := IDFor(req)
	second := IDFor(req)
	if first != second {
		t.Fatalf("idempotent IDs differ: %q != %q", first, second)
	}
	if _, err := uuid.Parse(string(first)); err != nil {
		t.Fatalf("IDFor returned a non-UUID %q: %v", first, err)
	}
	if random := IDFor(filament.RunRequest{}); random == first {
		t.Fatalf("random ID unexpectedly matched deterministic ID %q", first)
	} else if _, err := uuid.Parse(string(random)); err != nil {
		t.Fatalf("IDFor returned a non-UUID random ID %q: %v", random, err)
	}
}
