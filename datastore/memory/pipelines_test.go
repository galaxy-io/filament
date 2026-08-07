package memory

import (
	"context"
	"errors"
	"testing"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func newPipeline(id, name string) *ingestionv1.Pipeline {
	return &ingestionv1.Pipeline{Id: id, TenantId: "t1", Name: name}
}

func TestStore_ListPipelinesExcludesDeletedByDefault(t *testing.T) {
	ctx := context.Background()
	store := New()
	for _, p := range []*ingestionv1.Pipeline{newPipeline("pipe-1", "live"), newPipeline("pipe-2", "doomed")} {
		if _, err := store.CreatePipeline(ctx, p); err != nil {
			t.Fatalf("CreatePipeline: %v", err)
		}
	}
	if err := store.DeletePipeline(ctx, "pipe-2"); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}

	listed, err := store.ListPipelines(ctx, filament.PipelineFilter{Tenant: "t1"})
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(listed) != 1 || listed[0].GetId() != "pipe-1" {
		t.Fatalf("expected only the live pipeline, got %+v", listed)
	}
}

func TestStore_ListPipelinesIncludeDeleted(t *testing.T) {
	ctx := context.Background()
	store := New()
	for _, p := range []*ingestionv1.Pipeline{
		newPipeline("pipe-1", "live"),
		newPipeline("pipe-2", "doomed"),
		newPipeline("pipe-3", "live-too"),
	} {
		if _, err := store.CreatePipeline(ctx, p); err != nil {
			t.Fatalf("CreatePipeline: %v", err)
		}
	}
	if err := store.DeletePipeline(ctx, "pipe-2"); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}

	listed, err := store.ListPipelines(ctx, filament.PipelineFilter{Tenant: "t1", IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(listed) != 3 {
		t.Fatalf("expected the tombstone included, got %+v", listed)
	}
	if listed[1].GetDeletedAt() == 0 {
		t.Fatalf("expected deleted_at on the tombstone, got %+v", listed[1])
	}
	if listed[0].GetDeletedAt() != 0 || listed[2].GetDeletedAt() != 0 {
		t.Fatalf("expected deleted_at zero on live pipelines, got %+v", listed)
	}
	for _, p := range listed {
		if p.GetCreatedAt() == 0 {
			t.Fatalf("expected created_at set, got %+v", p)
		}
	}
	for i, want := range []string{"pipe-1", "pipe-2", "pipe-3"} {
		if listed[i].GetId() != want {
			t.Fatalf("expected ID-sorted merge, got %+v", listed)
		}
	}
}

func TestStore_DeletedPipelineInvisibleToLiveReads(t *testing.T) {
	ctx := context.Background()
	store := New()
	if _, err := store.CreatePipeline(ctx, newPipeline("pipe-1", "doomed")); err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	if err := store.DeletePipeline(ctx, "pipe-1"); err != nil {
		t.Fatalf("DeletePipeline: %v", err)
	}

	if _, err := store.LoadPipeline(ctx, "pipe-1"); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
	if _, err := store.CreatePipelineVersion(ctx, "pipe-1", &ingestionv1.PipelineVersion{}); !errors.Is(err, filament.ErrNotFound) {
		t.Fatalf("expected ErrNotFound creating a version on a deleted pipeline, got %v", err)
	}
	listed, err := store.ListPipelines(ctx, filament.PipelineFilter{})
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(listed) != 0 {
		t.Fatalf("expected an unfiltered listing to omit the tombstone, got %+v", listed)
	}
}

func TestStore_RecreateDeletedPipelineIDDoesNotDuplicate(t *testing.T) {
	creators := map[string]func(context.Context, *Store, *ingestionv1.Pipeline) error{
		"CreatePipeline": func(ctx context.Context, s *Store, p *ingestionv1.Pipeline) error {
			_, err := s.CreatePipeline(ctx, p)
			return err
		},
		"CreatePipelineWithSchedule": func(ctx context.Context, s *Store, p *ingestionv1.Pipeline) error {
			_, err := s.CreatePipelineWithSchedule(ctx, p, nil)
			return err
		},
	}

	for name, create := range creators {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			store := New()
			if err := create(ctx, store, newPipeline("pipe-1", "first")); err != nil {
				t.Fatalf("create: %v", err)
			}
			if err := store.DeletePipeline(ctx, "pipe-1"); err != nil {
				t.Fatalf("DeletePipeline: %v", err)
			}
			if err := create(ctx, store, newPipeline("pipe-1", "second")); err != nil {
				t.Fatalf("create reusing a deleted ID: %v", err)
			}

			listed, err := store.ListPipelines(ctx, filament.PipelineFilter{IncludeDeleted: true})
			if err != nil {
				t.Fatalf("ListPipelines: %v", err)
			}
			if len(listed) != 1 {
				t.Fatalf("expected the tombstone replaced, got %+v", listed)
			}
			if listed[0].GetName() != "second" {
				t.Fatalf("expected the live pipeline to win, got %+v", listed[0])
			}
		})
	}
}

func TestStore_ListPipelinesIncludeDeletedFiltersByTenant(t *testing.T) {
	ctx := context.Background()
	store := New()
	other := newPipeline("pipe-other", "other")
	other.TenantId = "t2"
	for _, p := range []*ingestionv1.Pipeline{newPipeline("pipe-1", "mine"), other} {
		if _, err := store.CreatePipeline(ctx, p); err != nil {
			t.Fatalf("CreatePipeline: %v", err)
		}
	}
	for _, id := range []string{"pipe-1", "pipe-other"} {
		if err := store.DeletePipeline(ctx, id); err != nil {
			t.Fatalf("DeletePipeline: %v", err)
		}
	}

	listed, err := store.ListPipelines(ctx, filament.PipelineFilter{Tenant: "t1", IncludeDeleted: true})
	if err != nil {
		t.Fatalf("ListPipelines: %v", err)
	}
	if len(listed) != 1 || listed[0].GetId() != "pipe-1" {
		t.Fatalf("expected another tenant's tombstone excluded, got %+v", listed)
	}
}
