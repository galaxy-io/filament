<img src=".github/filament.svg" alt="Filament" width="360" />

Filament moves data from sources to sinks with full, incremental, and CDC replication. Every batch is integrity checked on both the read and write side, and interrupted runs resume from checkpoints. It is written in Go, and its major pieces are pluggable. Connectors, the data store, and the event bus are interfaces with swappable implementations.

## Architecture

Filament is three components over a data store and an event bus.

- `server` serves the API and web UI and owns connections, pipelines, and run requests
- `control-plane` dispatches requested runs and tracks run state
- `worker` executes a single run from extraction to verified write

The data store holds durable state such as connections, pipelines, runs, and checkpoints. The event bus carries the events the components communicate through, rather than the components calling each other. Workers emit events as a run progresses and the control plane folds them into run state. Within a run, the pipeline handles batching, writing, integrity verification, and checkpoint advancement, so interrupted runs resume where they left off.

## Deploying

Filament runs on Kubernetes through the Helm chart. The chart deploys the server and control plane and can provision the backing data store and event bus or point at existing instances. Each run executes as its own Job. See [charts/filament](charts/filament/README.md) for installation and configuration.

Filament also embeds as a library. Connectors self-register via blank imports.

```go
import (
	"github.com/galaxy-io/filament/app"

	_ "github.com/galaxy-io/filament/connectors/postgres"
	_ "github.com/galaxy-io/filament/connectors/stdout"
)

func main() { log.Fatal(app.Run(context.Background())) }
```

## Writing a connector

A source implements `Spec`, `Validate`, `Configure`, `Extract`, and `Teardown` in [source.go](source.go). A sink implements `Spec`, `Open`, `Apply`, `Commit`, and `Abort` in [sink.go](sink.go). Optional interfaces add checkpointed resume, CDC, discovery, rate limits, staged transactions, upserts, and typed DDL. The engine detects them at runtime.

## Contributing

Run `brew bundle` to install the toolchain and make sure Docker is running.

```sh
just infra   # local data store + event bus via docker compose
just dev     # control plane, API server, and UI on port 5173
just test    # unit tests (just test-integration for e2e, needs Docker)
```

Run `just format`, `just lint`, and `just test` before opening a PR. Filament is pre-1.0 and APIs may change.
