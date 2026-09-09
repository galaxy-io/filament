# testcontainers

Ephemeral backing-service containers for integration and end-to-end tests,
seed operations, and benchmarks. Test-only helpers carry
`//go:build integration || e2e` so everyday `go test ./...` stays fast and
container-engine-free.

## Requirements

- Docker or configured Podman (running). See the [local development guide](../../docs/pages/guides/contributing/local-development.mdx)
- Go 1.26.4+
- `duckdb` on PATH for TPC-H seeds (`brew install duckdb`)

## Image pinning

Image pins live in [`internal/envfile/images.env`](internal/envfile/images.env)
and are shared with Compose. Repository-root `docker/.env` overrides them.
Exported image variables take precedence.

## Running integration tests

From the repository root:

```sh
just test-integration       # service-level suite
just test-e2e               # process, data-lake, and k3s suite
just test-container-runtime # automatic cleanup checks
```

Integration packages live under `tests/integration/...` and use
`//go:build integration`. End-to-end packages live under `tests/e2e/...` and
use `//go:build e2e`. Both recipes discover their complete suite recursively,
so adding a package under the appropriate tree needs no recipe change.

Exclusive containers are terminated through `t.Cleanup`. Shared containers are removed by Ryuk when the test session ends. Keep Ryuk enabled.

## Containers

### Postgres

```go
pg := testcontainers.Postgres(t)
pool := pg.Pool()   // *pgxpool.Pool
dsn  := pg.DSN()    // "postgres://test:test@localhost:<port>/test?sslmode=disable"
```

Options:

```go
pg := testcontainers.Postgres(t,
    testcontainers.WithDatabase("mydb"),
    testcontainers.WithUsername("user"),
    testcontainers.WithPassword("pass"),
    testcontainers.WithImage("docker.io/library/postgres:15"),
)
```

#### Snapshot / Restore

Seed once, snapshot, then restore between tests — a filesystem-level `CREATE DATABASE … TEMPLATE`, not a row-by-row reload. Significantly faster than re-seeding per test.

```go
func TestFoo(t *testing.T) {
    pg := testcontainers.Postgres(t)

    // seed
    _, _ = pg.Pool().Exec(ctx, "INSERT INTO …")

    pg.Snapshot(t)   // mark restore point

    t.Run("case A", func(t *testing.T) {
        pg.Restore(t)            // reset to snapshot
        pool := pg.Pool()        // always re-read pool after Restore
        // … test …
    })

    t.Run("case B", func(t *testing.T) {
        pg.Restore(t)
        pool := pg.Pool()
        // … test …
    })
}
```

> **Note:** `Pool()` returns a new handle after each `Snapshot` or `Restore`. Any handle taken before the call is closed and must not be used.

### MySQL

```go
mysql := testcontainers.MySQLContainer(t)
mysql.DSN     // app connection to the test database
mysql.DB      // connected *sql.DB
mysql.RootDB  // admin handle for lifecycle assertions
```

The server has row binlogs, GTIDs, and local infile enabled so the same
container exercises snapshot, incremental, CDC, and sink loading behavior.

### NATS

```go
n := testcontainers.NATSContainer(t)
n.URL   // "nats://localhost:<port>"
n.Conn  // *nats.Conn (connected, closed on cleanup)
```

### Kubernetes (k3s)

```go
k := testcontainers.K3sCluster(t)
k.KubeconfigPath // temporary kubeconfig for the cluster
k.Clientset      // connected kubernetes.Interface
```

The k3s container runs privileged and is heavier than the other helpers. Prefer
one cluster per test package.

### Redis

```go
r := testcontainers.RedisContainer(t)
r.Addr    // "localhost:<port>"
r.Client  // *redis.Client (connected, closed on cleanup)
```

### MinIO

```go
m := testcontainers.MinIOContainer(t)
m.Endpoint  // "localhost:<port>"
m.Client    // *minio.Client (connected, closed on cleanup)
```

### Trino

```go
tr := testcontainers.TrinoContainer(t)
tr.DSN  // "http://test@localhost:<port>?catalog=memory&schema=default"
tr.DB   // *sql.DB (memory catalog, closed on cleanup)
```

### Trino + Iceberg + MinIO (data lake)

A 3-container stack — Trino querying an Iceberg REST catalog backed by MinIO.
Slow to boot. Used by the e2e data-lake suite.

```go
dl := testcontainers.TrinoDataLake(t)
dl.DB      // *sql.DB on the "iceberg" catalog (data in MinIO)
dl.Bucket  // MinIO bucket backing the warehouse

dl.DB.ExecContext(ctx, "CREATE SCHEMA iceberg.demo")
dl.DB.ExecContext(ctx, "CREATE TABLE iceberg.demo.t AS SELECT 1 id")
```

## Seeding

The `seed` package defines a `Spec` (shape + size) and a registry of named scenarios. Backend drivers (Postgres, Redis, …) implement `SeederFunc`, `DropFunc`, and `VerifyFunc`.

### Built-in scenarios

| Name | Backend | Tables | Approximate total rows |
|------|---------|--------|------------------------|
| `multitenant-sm` | postgres | 2 | 2,000 |
| `multitenant-lg` | postgres | 10 | 1,000,000 |
| `tpch-sf0.1` | postgres | 8 | ~867K |
| `tpch-sf1` | postgres | 8 | ~8.7M |
| `tpch-sf5` | postgres | 8 | ~43.3M |

Activate built-in scenarios with a blank import:

```go
import _ "github.com/galaxy-io/filament/tests/testcontainers/seed/scenarios" // multitenant
import _ "github.com/galaxy-io/filament/tests/testcontainers/seed/tpch"      // TPC-H
```

### Using a scenario in a test

```go
import (
    "github.com/galaxy-io/filament/tests/testcontainers/seed"
    _ "github.com/galaxy-io/filament/tests/testcontainers/seed/scenarios"
)

func TestWithSeed(t *testing.T) {
    pg := testcontainers.Postgres(t)

    sc, _ := seed.Get("multitenant-sm")
    _, err := sc.Seeders["postgres"](ctx, pg.DSN())
    if err != nil {
        t.Fatal(err)
    }

    pg.Snapshot(t)
    // … tests using pg.Pool() …
}
```

### Scale via environment

```sh
SEED_SCALE=large go test -tags integration ./...
```

`seed.Default()` returns `seed.Small` unless `SEED_SCALE=large`, in which case it returns `seed.Large`.

### TPC-H

TPC-H seeding uses DuckDB to generate CSVs and loads them into Postgres via `COPY FROM STDIN`. Register additional scale factors at runtime:

```go
import "github.com/galaxy-io/filament/tests/testcontainers/seed/tpch"

tpch.RegisterSF(0.01) // fast smoke test SF
```

## Benchmarks

Combine a container, a seed, and snapshot/restore for repeatable benchmark state:

```go
func BenchmarkQuery(b *testing.B) {
    pg := testcontainers.Postgres(b)

    sc, _ := seed.Get("multitenant-sm")
    sc.Seeders["postgres"](ctx, pg.DSN())
    pg.SnapshotCtx(ctx)

    b.ResetTimer()
    for i := 0; i < b.N; i++ {
        pg.RestoreCtx(ctx)
        pool := pg.Pool()
        // … benchmark operation …
    }
}
```

Run benchmarks with the integration tag:

```sh
go test -tags integration -bench=. -benchtime=10s ./...
```

## Long-lived containers (CLI / `gx`)

The `container` sub-package starts containers outside of a test context and persists their state to `~/.config/gx/containers.json` so they survive across CLI invocations. Ryuk is disabled for these containers — only an explicit stop call removes them.

```go
import "github.com/galaxy-io/filament/tests/testcontainers/container"

e, err := container.StartPostgres(ctx, container.PostgresOptions{
    Name:    "dev-pg",
    Network: "gx",
})
// e.DSN — connection string

container.StopContainer(ctx, "dev-pg")
```

State entries are listed via `container.ListEntries()` and looked up by name with `container.GetEntry(name)`.
