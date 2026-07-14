// Command ingestiond is the reference Filament service. It ships a curated set
// of connectors (each self-registers via a blank import) plus the dep-free
// built-in sample source and stdout sink, then hands off to app.Run.
package main

import (
	"context"
	"log"

	ingestion "github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/app"
	"github.com/galaxy-io/filament/connectors/sample"
	"github.com/galaxy-io/filament/connectors/stdout"
	"github.com/galaxy-io/filament/registry"
	"github.com/galaxy-io/filament/ui"

	// Curated connector set — self-register via init():
	_ "github.com/galaxy-io/filament/connectors/http"
	_ "github.com/galaxy-io/filament/connectors/iceberg"
	_ "github.com/galaxy-io/filament/connectors/object"
	_ "github.com/galaxy-io/filament/connectors/postgres"
)

func main() {
	// Dep-free built-ins ship with the reference binary; register explicitly.
	registry.RegisterSource("sample", func() ingestion.Source { return sample.New() })
	registry.RegisterSink("stdout", func() ingestion.Sink { return stdout.New() })

	// Defaults to the in-process bus and in-memory store. Swap either without
	// touching the wiring, e.g.:
	//	app.Run(ctx, app.WithBus(natsBus), app.WithDataStore(pgStore))
	//
	// The web UI is a placeholder page unless the binary is built with
	// -tags embedui after building ui/dist (see ui/README.md).
	if err := app.Run(context.Background(), app.WithUI(ui.Handler())); err != nil {
		log.Fatal(err)
	}
}
