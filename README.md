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

Common tasks are driven by [`just`](https://github.com/casey/just); CI runs the
same recipes, so a green run locally is a green run in CI:

```sh
just binaries         # build linux binaries into bin/
just images           # build docker images
just format           # apply gofumpt + goimports to every module (settings in .golangci.yaml)
just format-check     # check formatting without writing
just lint             # run golangci-lint with auto-fixes across every module
just lint-check       # run golangci-lint without fixing (what CI runs)
just test             # run unit tests in every module except tests/
just test-integration # run the integration/e2e suite (requires Docker)
```
