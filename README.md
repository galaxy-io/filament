# Filament

Filament is a Go ingestion runtime for moving records from sources into sinks with
checkpointing, batching, integrity events, and pluggable connectors.

## Repository Layout

- `app/`: public composition root for running the service.
- `api/`: ConnectRPC protobuf API and generated Go bindings.
- `cmd/ingestiond/`: example service binary.
- `connectors/`: optional source and sink implementations.
- `eventbus/`: event transport interfaces and local implementations.
- `pipeline/`: batching, writing, and integrity verification loop.
- `registry/`: provider registration and lookup.
- `server/`: ConnectRPC service handlers.
- `tests/`: integration and end-to-end test modules.

## Quick Start

```go
package main

import (
	"context"
	"log"

	"github.com/galaxy-io/filament/app"
	_ "github.com/galaxy-io/filament/connectors/http"
	_ "github.com/galaxy-io/filament/connectors/postgres"
)

func main() {
	if err := app.Run(context.Background()); err != nil {
		log.Fatal(err)
	}
}

// Eventually also without running the entire service
result, err := runner.Run(ctx, runner.Config{
  SourceConfig:   filament.NewConfig(pgsource.New(), map[string]any{"dsn": pgDSN}),
  SinkConfig:     filament.NewConfig(mysqlsink.New(), map[string]any{"dsn": mysqlDSN}),
  Resources:      []string{"users"},
})
```

## Development

The repo uses a Go workspace so optional connectors can carry their heavy
dependencies independently:

```sh
go work sync
go test ./...
```

Some integration tests require Docker through `testcontainers-go`.
