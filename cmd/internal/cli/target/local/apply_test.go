package local

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
	"github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/identity"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/server"
	"github.com/galaxy-io/filament/worker"
)

type versionedTestSource struct {
	filament.Source
	version string
}

func (s *versionedTestSource) Spec() filament.ConnectorSpec {
	name := "example"
	if s.version != "" {
		name += "@" + s.version
	}
	return filament.ConnectorSpec{Name: name, Version: s.version, Config: filament.ConfigSchema{Fields: []filament.ConfigField{{Name: "host", Type: filament.FieldString, Scope: filament.ScopeConnection}}}}
}
func (s *versionedTestSource) Validate(filament.Config) error { return nil }

func TestApplyUnversionedDocumentPreservesPinnedConnection(t *testing.T) {
	ctx := context.Background()
	db := sqlite.NewMemory()
	t.Cleanup(func() { _ = db.Close() })
	store := Store{Path: filepath.Join(t.TempDir(), "filament.yaml")}
	writeDocument := func(host string) {
		t.Helper()
		if err := os.WriteFile(store.Path, []byte("version: 1\nsources:\n  input:\n    type: example\n    config:\n      host: "+host+"\n"), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	targetFor := func(defaultVersion string) *Target {
		t.Helper()
		sources := registry.NewSources()
		for _, version := range []string{"v1", "v2"} {
			sources.Register("example@"+version, func() filament.Source { return &versionedTestSource{version: version} })
		}
		if defaultVersion == "" {
			sources.Register("example", func() filament.Source { return &versionedTestSource{} })
		} else {
			sources.RegisterAlias("example", "example@"+defaultVersion)
		}
		api := server.New(worker.Local(sources, registry.NewSinks()), db, nil, nil)
		_, handler := ingestionv1connect.NewIngestionServiceHandler(api)
		httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			handler.ServeHTTP(w, r.WithContext(identity.WithTenant(r.Context(), filament.DefaultTenantID)))
		}))
		t.Cleanup(httpServer.Close)
		return NewTarget(store, remote.NewTarget(remote.Options{Endpoint: httpServer.URL}), nil)
	}
	writeDocument("before")
	target := targetFor("v1")
	if err := target.Apply(ctx); err != nil {
		t.Fatal(err)
	}
	first, err := target.GetConnection(ctx, "source", "input")
	if err != nil {
		t.Fatal(err)
	}
	if first.Type != "example@v1" {
		t.Fatalf("connector=%q", first.Type)
	}
	if err := target.Apply(ctx); err != nil {
		t.Fatalf("unchanged reapply: %v", err)
	}
	unchanged, err := target.GetConnection(ctx, "source", "input")
	if err != nil {
		t.Fatal(err)
	}
	if unchanged.Metadata.Revision != first.Metadata.Revision {
		t.Fatal("unchanged document revised connection")
	}

	target = targetFor("v2")
	writeDocument("after")
	if err := target.Apply(ctx); err != nil {
		t.Fatalf("reapply after default change: %v", err)
	}
	updated, err := target.GetConnection(ctx, "source", "input")
	if err != nil {
		t.Fatal(err)
	}
	if updated.Type != "example@v1" || updated.Config["host"] != "after" || updated.Metadata.ID != first.Metadata.ID {
		t.Fatalf("updated=%+v", updated)
	}

	// An explicit version change is still a connector change and must fail.
	if err := os.WriteFile(store.Path, []byte("version: 1\nsources:\n  input:\n    type: example@v2\n    config:\n      host: after\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := target.Apply(ctx); err == nil {
		t.Fatal("explicit connector version change accepted")
	}

	// A concrete bare registration is a different connector, even when a
	// versioned registration with the same base name is still available.
	target = targetFor("")
	writeDocument("rejected")
	if err := target.Apply(ctx); err == nil || !strings.Contains(err.Error(), `connector cannot change from "example@v1" to "example"`) {
		t.Fatalf("concrete connector change: %v", err)
	}
	preserved, err := target.GetConnection(ctx, "source", "input")
	if err != nil {
		t.Fatal(err)
	}
	if preserved.Type != updated.Type || preserved.Config["host"] != updated.Config["host"] || preserved.Metadata.Revision != updated.Metadata.Revision {
		t.Fatalf("rejected connector change modified connection: %+v", preserved)
	}
}
