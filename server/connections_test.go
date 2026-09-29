package server

import (
	"testing"

	"connectrpc.com/connect"
	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/registry"
)

func TestCreateConnectionPinsDefaultConnectorVersion(t *testing.T) {
	store := sqlite.NewMemory()
	sources := registry.NewSources()
	sources.Register("example@v1", func() filament.Source { return &catalogSource{name: "example@v1"} })
	sources.RegisterAlias("example", "example@v1")
	api := New(sources, registry.NewSinks(), store, nil, nil)
	created, err := api.CreateConnection(testCtx(), connect.NewRequest(&ingestionv1.CreateConnectionRequest{
		Kind: ingestionv1.ConnectorKind_CONNECTOR_KIND_SOURCE, Name: "example", Connector: "example",
	}))
	if err != nil {
		t.Fatal(err)
	}
	if got := created.Msg.GetConnection().GetConnector(); got != "example@v1" {
		t.Fatalf("response connector=%q", got)
	}
	stored, err := store.LoadConnection(testCtx(), filament.DefaultTenantID, created.Msg.GetConnection().GetId())
	if err != nil {
		t.Fatal(err)
	}
	if stored.Connector != "example@v1" {
		t.Fatalf("stored connector=%q", stored.Connector)
	}

	// A subsequent binary may select v2 by default. The persisted connection
	// must still select v1 without relying on that default alias.
	upgraded := registry.NewSources()
	for _, version := range []string{"v1", "v2"} {
		upgraded.Register("example@"+version, func() filament.Source { return &catalogSource{name: "example@" + version} })
	}
	upgraded.RegisterAlias("example", "example@v2")
	source, err := upgraded.Resolve(stored.Connector)
	if err != nil || source.Spec().Name != "example@v1" {
		t.Fatalf("pinned resolution=%v err=%v", source, err)
	}
}
