package memory

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

func TestListConnectionsSearchSortAndPage(t *testing.T) {
	ctx := context.Background()
	store := New()
	for _, connection := range []filament.Connection{
		{ID: "c-1", Tenant: "t-1", Kind: filament.ConnectorKindSource, Name: "Alpha Warehouse", Connector: "postgres"},
		{ID: "c-2", Tenant: "t-1", Kind: filament.ConnectorKindSource, Name: "Zulu Warehouse", Connector: "postgres"},
		{ID: "c-3", Tenant: "t-1", Kind: filament.ConnectorKindSource, Name: "Unrelated", Connector: "mysql"},
	} {
		if _, err := store.CreateConnection(ctx, connection); err != nil {
			t.Fatal(err)
		}
	}

	connections, total, err := store.ListConnections(ctx, filament.ConnectionFilter{
		Tenant: "t-1", ListOptions: filament.ListOptions{
			Search: "ware", SortBy: "name", SortDescending: true, Limit: 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(connections) != 1 || connections[0].Name != "Zulu Warehouse" {
		t.Fatalf("connections = %+v, total = %d", connections, total)
	}
}

func TestListPipelinesAndVersionsSearchSortAndPage(t *testing.T) {
	ctx := context.Background()
	store := New()
	for _, pipeline := range []*ingestionv1.Pipeline{
		{Id: "p-1", TenantId: "t-1", Name: "Alpha Sync", Description: "warehouse orders"},
		{Id: "p-2", TenantId: "t-1", Name: "Zulu Sync", Description: "warehouse customers"},
		{Id: "p-3", TenantId: "t-1", Name: "Other", Description: "billing"},
	} {
		if _, err := store.CreatePipeline(ctx, pipeline); err != nil {
			t.Fatal(err)
		}
	}

	pipelines, total, err := store.ListPipelines(ctx, filament.PipelineFilter{
		Tenant: "t-1", ListOptions: filament.ListOptions{
			Search: "ware", SortBy: "name", Limit: 1, Offset: 1,
		},
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(pipelines) != 1 || pipelines[0].GetName() != "Zulu Sync" {
		t.Fatalf("pipelines = %+v, total = %d", pipelines, total)
	}

	for range 2 {
		if _, err := store.CreatePipelineVersion(ctx, "p-1", &ingestionv1.PipelineVersion{}); err != nil {
			t.Fatal(err)
		}
	}
	versions, versionTotal, err := store.ListPipelineVersions(ctx, filament.PipelineVersionFilter{PipelineID: "p-1"})
	if err != nil {
		t.Fatal(err)
	}
	if versionTotal != 2 || versions[0].GetVersion() != 2 || versions[1].GetVersion() != 1 {
		t.Fatalf("versions = %+v, total = %d", versions, versionTotal)
	}
}

func TestListRunsSearchesAndSortsByPipelineName(t *testing.T) {
	ctx := context.Background()
	store := New()
	for _, pipeline := range []*ingestionv1.Pipeline{
		{Id: "p-a", TenantId: "t-1", Name: "Alpha Pipeline"},
		{Id: "p-z", TenantId: "t-1", Name: "Zulu Pipeline"},
	} {
		if _, err := store.CreatePipeline(ctx, pipeline); err != nil {
			t.Fatal(err)
		}
	}
	now := time.Now()
	for _, run := range []filament.RunState{
		{Run: "r-a", Tenant: "t-1", Request: filament.RunRequest{PipelineID: "p-a"}, CreatedAt: now},
		{Run: "r-z", Tenant: "t-1", Request: filament.RunRequest{PipelineID: "p-z"}, CreatedAt: now.Add(time.Second)},
	} {
		if err := store.SaveRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}
	runs, total, err := store.ListRuns(ctx, filament.RunFilter{
		Tenant: "t-1", Search: "pipe", SortBy: "name", SortDescending: true, Limit: 1,
	})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 || len(runs) != 1 || runs[0].Run != "r-z" {
		t.Fatalf("runs = %+v, total = %d", runs, total)
	}
}

func TestListRunsDefaultSortEffectiveTime(t *testing.T) {
	ctx := context.Background()
	store := New()
	now := time.Now()
	ended := now.Add(-time.Hour)
	for _, run := range []filament.RunState{
		{Run: "r-completed", Tenant: "t-1", Status: filament.RunCompleted, StartedAt: now.Add(-2 * time.Hour), CreatedAt: now.Add(-2 * time.Hour)},
		{Run: "r-running", Tenant: "t-1", Status: filament.RunRunning, StartedAt: now.Add(-5 * time.Minute), CreatedAt: now.Add(-5 * time.Minute)},
		{Run: "r-cancelled", Tenant: "t-1", Status: filament.RunCanceled, RequestedAt: now.Add(-time.Hour), EndedAt: &ended, CreatedAt: now.Add(-time.Hour)},
		// The regression shape: pre-created hours ago, firing in the future. It
		// must sort by its fire time, not sink to its creation time.
		{Run: "r-scheduled", Tenant: "t-1", Status: filament.RunScheduled, ScheduledAt: now.Add(time.Hour), CreatedAt: now.Add(-3 * time.Hour)},
	} {
		if err := store.SaveRun(ctx, run); err != nil {
			t.Fatal(err)
		}
	}

	runs, total, err := store.ListRuns(ctx, filament.RunFilter{Tenant: "t-1", SortDescending: true})
	if err != nil {
		t.Fatal(err)
	}
	want := []filament.RunID{"r-scheduled", "r-running", "r-cancelled", "r-completed"}
	if total != len(want) || len(runs) != len(want) {
		t.Fatalf("runs = %+v, total = %d", runs, total)
	}
	for i, id := range want {
		if runs[i].Run != id {
			t.Fatalf("descending order[%d] = %s, want %s", i, runs[i].Run, id)
		}
	}

	runs, _, err = store.ListRuns(ctx, filament.RunFilter{Tenant: "t-1"})
	if err != nil {
		t.Fatal(err)
	}
	for i, id := range want {
		if runs[len(want)-1-i].Run != id {
			t.Fatalf("ascending order[%d] = %s, want %s", len(want)-1-i, runs[len(want)-1-i].Run, id)
		}
	}
}
