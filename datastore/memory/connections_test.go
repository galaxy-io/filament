package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
)

func newConnection(id, name string, kind filament.ConnectorKind) filament.Connection {
	return filament.Connection{ID: id, Tenant: "t1", Kind: kind, Name: name, Connector: "postgres"}
}

func TestStore_ListConnectionsExcludesDeletedByDefault(t *testing.T) {
	ctx := context.Background()
	store := New()
	for _, c := range []filament.Connection{
		newConnection("conn-1", "live", filament.ConnectorKindSource),
		newConnection("conn-2", "doomed", filament.ConnectorKindSource),
	} {
		if _, err := store.CreateConnection(ctx, c); err != nil {
			t.Fatalf("CreateConnection: %v", err)
		}
	}
	if err := store.DeleteConnection(ctx, "conn-2"); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}

	listed, err := store.ListConnections(ctx, filament.ConnectionFilter{Tenant: "t1"})
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "conn-1" {
		t.Fatalf("expected only the live connection, got %+v", listed)
	}
}

func TestStore_ListConnectionsIncludeDeleted(t *testing.T) {
	ctx := context.Background()
	store := New()
	// conn-2 sorts after conn-1 but is the one tombstoned, so a listing that
	// merges both maps must still come back ID-sorted.
	for _, c := range []filament.Connection{
		newConnection("conn-1", "live", filament.ConnectorKindSource),
		newConnection("conn-2", "doomed", filament.ConnectorKindSource),
		newConnection("conn-3", "live-too", filament.ConnectorKindSource),
	} {
		if _, err := store.CreateConnection(ctx, c); err != nil {
			t.Fatalf("CreateConnection: %v", err)
		}
	}
	if err := store.DeleteConnection(ctx, "conn-2"); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}

	listed, err := store.ListConnections(ctx, filament.ConnectionFilter{Tenant: "t1", IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected the tombstone included, got %+v", listed)
	}
	for i, want := range []string{"conn-1", "conn-2", "conn-3"} {
		if listed[i].ID != want {
			t.Fatalf("expected ID-sorted merge, got %+v", listed)
		}
	}
}

func TestStore_DeletedConnectionInvisibleToLiveReads(t *testing.T) {
	ctx := context.Background()
	store := New()
	if _, err := store.CreateConnection(ctx, newConnection("conn-1", "doomed", filament.ConnectorKindSource)); err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	if err := store.DeleteConnection(ctx, "conn-1"); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}

	if _, err := store.LoadConnection(ctx, "conn-1"); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	listed, err := store.ListConnections(ctx, filament.ConnectionFilter{})
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected an unfiltered listing to omit the tombstone, got %+v", listed)
	}
}

func TestStore_RecreateDeletedConnectionIDDoesNotDuplicate(t *testing.T) {
	ctx := context.Background()
	store := New()
	if _, err := store.CreateConnection(ctx, newConnection("conn-1", "first", filament.ConnectorKindSource)); err != nil {
		t.Fatalf("CreateConnection: %v", err)
	}
	if err := store.DeleteConnection(ctx, "conn-1"); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}
	if _, err := store.CreateConnection(ctx, newConnection("conn-1", "second", filament.ConnectorKindSource)); err != nil {
		t.Fatalf("CreateConnection reusing a deleted ID: %v", err)
	}

	listed, err := store.ListConnections(ctx, filament.ConnectionFilter{IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(listed) != 1 {
		t.Fatalf("expected the tombstone replaced, got %+v", listed)
	}
	if listed[0].Name != "second" {
		t.Fatalf("expected the live connection to win, got %+v", listed[0])
	}
}

func TestStore_ListConnectionsIncludeDeletedRespectsKindFilter(t *testing.T) {
	ctx := context.Background()
	store := New()
	for _, c := range []filament.Connection{
		newConnection("conn-source", "src", filament.ConnectorKindSource),
		newConnection("conn-sink", "snk", filament.ConnectorKindSink),
	} {
		if _, err := store.CreateConnection(ctx, c); err != nil {
			t.Fatalf("CreateConnection: %v", err)
		}
	}
	if err := store.DeleteConnection(ctx, "conn-sink"); err != nil {
		t.Fatalf("DeleteConnection: %v", err)
	}

	listed, err := store.ListConnections(ctx, filament.ConnectionFilter{Kind: filament.ConnectorKindSource, IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "conn-source" {
		t.Fatalf("expected a deleted sink excluded from a source listing, got %+v", listed)
	}
}

func TestStore_ListConnectionsIncludeDeletedFiltersByTenant(t *testing.T) {
	ctx := context.Background()
	store := New()
	other := newConnection("conn-other", "other", filament.ConnectorKindSource)
	other.Tenant = "t2"
	for _, c := range []filament.Connection{newConnection("conn-1", "mine", filament.ConnectorKindSource), other} {
		if _, err := store.CreateConnection(ctx, c); err != nil {
			t.Fatalf("CreateConnection: %v", err)
		}
	}
	for _, id := range []string{"conn-1", "conn-other"} {
		if err := store.DeleteConnection(ctx, id); err != nil {
			t.Fatalf("DeleteConnection: %v", err)
		}
	}

	listed, err := store.ListConnections(ctx, filament.ConnectionFilter{Tenant: "t1", IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListConnections: %v", err)
	}
	if len(listed) != 1 || listed[0].ID != "conn-1" {
		t.Fatalf("expected another tenant's tombstone excluded, got %+v", listed)
	}
}
