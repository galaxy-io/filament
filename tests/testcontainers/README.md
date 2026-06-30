# testcontainers

Ephemeral backing-service containers for integration tests, seed operations, and benchmarks. Gated behind `//go:build integration` so everyday `go test ./...` stays fast and Docker-free.

## Requirements

- Docker (running)
- Go 1.25+
- `duckdb` on PATH for TPC-H seeds (`brew install duckdb`)

## Image pinning

Images are read from `docker/.env` in the nearest parent containing `go.mod`. The keys are:

```
POSTGRES_IMAGE=postgres:16
NATS_IMAGE=nats:2
REDIS_IMAGE=redis:7
MINIO_IMAGE=minio/minio:latest
TRINO_IMAGE=trinodb/trino:latest
ICEBERG_REST_IMAGE=apache/iceberg-rest-fixture:latest
```

This keeps testcontainers and your dev compose stack on the same versions — bump a tag once and both pick it up. Fallback defaults (`postgres:latest`, etc.) are used when the file is absent or missing a key.

## Running integration tests

```sh
go test -tags integration ./...
```

Containers start, run, and are terminated automatically via `t.Cleanup`. No manual teardown needed.

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
    testcontainers.WithImage("postgres:15"),
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

### NATS

```go
n := testcontainers.NATSContainer(t)
n.URL   // "nats://localhost:<port>"
n.Conn  // *nats.Conn (connected, closed on cleanup)
```

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

A 3-container stack — Trino querying an Iceberg catalog (in-memory
`apache/iceberg-rest-fixture`) backed by MinIO. Slow to boot; integration only.

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

| Name | Backend | Tables | Rows/table |
|------|---------|--------|------------|
| `multitenant-sm` | postgres | 2 | 1,000 |
| `multitenant-lg` | postgres | 10 | 100,000 |
| `tpch-sf0.1` | postgres | 8 | ~75K avg |
| `tpch-sf1` | postgres | 8 | ~750K avg |
| `tpch-sf5` | postgres | 8 | ~3.75M avg |

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
