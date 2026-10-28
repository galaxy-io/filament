package local

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/api/ingestion/v1/ingestionv1connect"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/target/remote"
	"github.com/galaxy-io/filament/connectors/stdout"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/identity"
	"github.com/galaxy-io/filament/registry"
	secretenv "github.com/galaxy-io/filament/secret/env"
	"github.com/galaxy-io/filament/server"
	"github.com/galaxy-io/filament/worker"
)

type passwordDiscoverySource struct{ filament.Source }

func (*passwordDiscoverySource) Spec() filament.ConnectorSpec {
	return filament.ConnectorSpec{Name: "password-fixture", Modes: []filament.ReadMode{filament.ModeFull},
		SourcePolicies: filament.SourcePolicies(filament.IngestionFullAppend),
		Config: filament.ConfigSchema{Fields: []filament.ConfigField{
			{Name: "password", Type: filament.FieldSecret, Required: true, Scope: filament.ScopeConnection},
		}}}
}

func (*passwordDiscoverySource) Validate(filament.Config) error { return nil }

func (*passwordDiscoverySource) Configure(_ context.Context, cfg filament.Config) error {
	if cfg.Secret("password") != "actual-password" {
		return fmt.Errorf("access denied")
	}
	return nil
}

func (*passwordDiscoverySource) Teardown(context.Context) error { return nil }

func (*passwordDiscoverySource) Discover(context.Context, filament.DiscoverOpts) (filament.DiscoverResult, error) {
	return filament.DiscoverResult{Resources: []filament.Resource{{Name: "users", Selectable: true}}}, nil
}

func (*passwordDiscoverySource) Schema(context.Context, string) (filament.RecordSchema, error) {
	return filament.RecordSchema{Resource: "users", PrimaryKey: []string{"id"}, Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalString},
	}}, nil
}

func TestDiscoverResolvesEnvironmentSecretReferences(t *testing.T) {
	t.Setenv("FPW_ENV", "actual-password")
	for _, reference := range []string{"env:FPW_ENV", "$FPW_ENV", "${FPW_ENV}"} {
		t.Run(reference, func(t *testing.T) {
			ctx := context.Background()
			store := sqlite.NewMemory()
			t.Cleanup(func() { _ = store.Close() })
			sources := registry.NewSources()
			sources.Register("password-fixture", func() filament.Source { return &passwordDiscoverySource{} })
			sinks := registry.NewSinks()
			sinks.Register("stdout", func() filament.Sink { return stdout.New() })
			api := server.New(worker.Local(sources, sinks), store, nil, nil, server.WithSecrets(secretenv.New()))
			_, handler := ingestionv1connect.NewIngestionServiceHandler(api)
			httpServer := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				handler.ServeHTTP(w, r.WithContext(identity.WithTenant(r.Context(), filament.DefaultTenantID)))
			}))
			t.Cleanup(httpServer.Close)
			target := NewTarget(Store{Path: filepath.Join(t.TempDir(), "filament.yaml")},
				remote.NewTarget(remote.Options{Endpoint: httpServer.URL}), secretenv.New())
			service := cliapp.NewService(target)
			connection, err := service.SaveConnection(ctx, cliapp.SaveConnectionRequest{
				Create: true, Kind: "source", Name: "envtest", Connector: "password-fixture",
				Config: cliapp.ConfigPatch{Values: map[string]any{"password": reference}},
			})
			if err != nil {
				t.Fatal(err)
			}
			stored, err := store.LoadConnection(ctx, filament.DefaultTenantID, connection.Metadata.ID)
			if err != nil {
				t.Fatal(err)
			}
			if stored.SecretRefs["password"] != "FPW_ENV" {
				t.Fatalf("secret ref = %q", stored.SecretRefs["password"])
			}
			if _, hasPassword := stored.Config["password"]; hasPassword {
				t.Fatalf("secret reference persisted as config: %#v", stored.Config)
			}
			result, err := service.DiscoverSource(ctx, cliapp.DiscoverSourceRequest{Source: "envtest"})
			if err != nil || len(result.Items) != 1 || result.Items[0].Name != "users" {
				t.Fatalf("saved discovery = %#v, %v", result, err)
			}
			result, err = service.DiscoverSource(ctx, cliapp.DiscoverSourceRequest{
				Connector: "password-fixture", Config: cliapp.ConfigPatch{Values: map[string]any{"password": reference}},
			})
			if err != nil || len(result.Items) != 1 {
				t.Fatalf("inline discovery = %#v, %v", result, err)
			}
			if _, err := service.SaveConnection(ctx, cliapp.SaveConnectionRequest{
				Create: true, Kind: "sink", Name: "out", Connector: "stdout",
			}); err != nil {
				t.Fatal(err)
			}
			resources := []string{"users"}
			if _, err := service.SavePipeline(ctx, cliapp.SavePipelineRequest{
				Create: true, Name: "copy", Source: "envtest", Sink: "out", Resources: &resources,
				SyncMode: "full", WriteMode: "append",
			}); err != nil {
				t.Fatalf("pipeline create: %v", err)
			}
		})
	}
}
