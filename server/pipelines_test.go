package server_test

import (
	"context"
	"testing"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/memory"
	"github.com/galaxy-io/filament/server"
)

func TestGetPipeline_WithVersions(t *testing.T) {
	ctx := context.Background()
	api := server.New(nil, nil, memory.New(), nil, nil)

	created, err := api.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{Name: "p"}))
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}
	id := created.Msg.GetPipeline().GetId()

	for range 2 {
		if _, err := api.CreatePipelineVersion(ctx, connect.NewRequest(&ingestionv1.CreatePipelineVersionRequest{
			PipelineId: id,
		})); err != nil {
			t.Fatalf("CreatePipelineVersion: %v", err)
		}
	}

	resp, err := api.GetPipeline(ctx, connect.NewRequest(&ingestionv1.GetPipelineRequest{Id: id}))
	if err != nil {
		t.Fatalf("GetPipeline: %v", err)
	}
	if got := resp.Msg.GetPipeline().GetCurrentVersionId(); got != 2 {
		t.Fatalf("CurrentVersionId = %d, want 2", got)
	}
	if got := resp.Msg.GetCurrentVersion().GetVersion(); got != 2 {
		t.Fatalf("CurrentVersion.Version = %d, want 2", got)
	}
	versions := resp.Msg.GetVersions()
	if len(versions) != 2 {
		t.Fatalf("len(Versions) = %d, want 2", len(versions))
	}
	if versions[0].GetVersion() != 2 || versions[1].GetVersion() != 1 {
		t.Fatalf("Versions order = [%d, %d], want [2, 1]", versions[0].GetVersion(), versions[1].GetVersion())
	}
}

func TestGetPipeline_NoVersions(t *testing.T) {
	ctx := context.Background()
	api := server.New(nil, nil, memory.New(), nil, nil)

	created, err := api.CreatePipeline(ctx, connect.NewRequest(&ingestionv1.CreatePipelineRequest{Name: "p"}))
	if err != nil {
		t.Fatalf("CreatePipeline: %v", err)
	}

	resp, err := api.GetPipeline(ctx, connect.NewRequest(&ingestionv1.GetPipelineRequest{Id: created.Msg.GetPipeline().GetId()}))
	if err != nil {
		t.Fatalf("GetPipeline: %v", err)
	}
	if resp.Msg.GetCurrentVersion() != nil {
		t.Fatalf("CurrentVersion = %v, want nil", resp.Msg.GetCurrentVersion())
	}
	if got := len(resp.Msg.GetVersions()); got != 0 {
		t.Fatalf("len(Versions) = %d, want 0", got)
	}
}

func TestGetPipeline_NotFound(t *testing.T) {
	ctx := context.Background()
	api := server.New(nil, nil, memory.New(), nil, nil)

	_, err := api.GetPipeline(ctx, connect.NewRequest(&ingestionv1.GetPipelineRequest{Id: "missing"}))
	if connect.CodeOf(err) != connect.CodeNotFound {
		t.Fatalf("code = %v, want NotFound", connect.CodeOf(err))
	}
}
