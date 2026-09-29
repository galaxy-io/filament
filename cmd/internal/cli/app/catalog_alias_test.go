package app

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
	_ "github.com/galaxy-io/filament/connectors/http"
	"github.com/galaxy-io/filament/registry"
)

func TestLegacyHTTPConnectionWorkflows(t *testing.T) {
	target := newMemoryTarget()
	for _, spec := range registry.DefaultSources.Specs() {
		target.catalog.Sources[spec.Name] = spec
	}
	target.document.Sources["input"] = model.Connection{Type: "github", Config: map[string]any{"token": "env:GITHUB_TOKEN", "organization": "example"}}
	target.document.Sinks["output"] = model.Connection{Type: "stdout"}
	target.document.Pipelines["copy"] = model.Pipeline{Source: model.PipelineNode{Ref: "input"}, Sink: model.PipelineNode{Ref: "output"}, SyncMode: "full", WriteMode: "replace"}
	service := NewService(target)
	ctx := context.Background()
	if err := ValidateDocument(target.document, target.catalog); err != nil {
		t.Fatal(err)
	}
	if _, err := service.SaveConnection(ctx, SaveConnectionRequest{Kind: "source", Name: "input", Config: ConfigPatch{Values: map[string]any{"organization": "updated"}}}); err != nil {
		t.Fatal(err)
	}
	if got := target.document.Sources["input"].Type; got != "github" {
		t.Fatalf("legacy connector changed to %q", got)
	}
	if _, err := service.DiscoverSource(ctx, DiscoverSourceRequest{Source: "input"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PrepareRun(ctx, RunRequest{Pipeline: "copy"}); err != nil {
		t.Fatal(err)
	}
	if _, err := service.PrepareRun(ctx, RunRequest{Inline: &InlineRun{
		Source: InlineConnector{Connector: "github", Config: target.document.Sources["input"].Config},
		Sink:   InlineConnector{Connector: "stdout"}, SyncMode: "full", WriteMode: "replace",
	}}); err != nil {
		t.Fatal(err)
	}
	// Catalog resolution must retain the actual schema, not merely recognize names.
	if target.catalog.Sources["github"].Maturity != filament.MaturityBeta {
		t.Fatal("alias lost catalog maturity")
	}
	bad := target.document.Sources["input"]
	bad.Config = map[string]any{"token": "env:GITHUB_TOKEN"}
	target.document.Sources["input"] = bad
	if err := ValidateDocument(target.document, target.catalog); err == nil {
		t.Fatal("missing organization accepted through alias")
	}
}
