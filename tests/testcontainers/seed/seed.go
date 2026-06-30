// Package seed defines the canonical cross-backend data shape and tooling for
// seeding deterministic, reproducible datasets into backing services.
//
// The central idea: a Spec describes WHAT to generate (shape, size, salt).
// Backend drivers (seed/postgres, seed/redis, …) decide HOW to fill their
// service efficiently — a postgres driver uses server-side generate_series;
// a redis driver may pipeline SET calls. The same Spec always produces the
// same data regardless of backend.
//
// Scenarios are named, describable Specs that live in a shared Registry so
// the gx CLI and any test can refer to them by name rather than hard-coding
// table counts and row sizes.
package seed

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"
)

// Row is the canonical cross-backend row shape. Backend drivers map these
// fields to their native types.
type Row struct {
	ID        string         // UUID as string
	Tenant    string         // "t0"–"t3"
	Seq       int64          // monotonic series index
	Amount    int            // derived from seq + salt
	Payload   map[string]any // {"n": seq, "h": md5(seq+salt)}
	UpdatedAt time.Time      // 2024-01-01 + seq seconds
}

// Spec describes a dataset: N tables of identical shape, each with
// RowsPerTable rows. Same Spec (including Salt) always yields identical data.
type Spec struct {
	Name         string
	Tables       int
	RowsPerTable int
	Salt         int64
}

// Standard size tiers. Use Default() to pick one from the environment.
var (
	Small = Spec{Name: "multitenant-sm", Tables: 2, RowsPerTable: 1_000, Salt: 1}
	Large = Spec{Name: "multitenant-lg", Tables: 10, RowsPerTable: 100_000, Salt: 1}
)

// Default returns Large when SEED_SCALE=large, otherwise Small.
func Default() Spec {
	if os.Getenv("SEED_SCALE") == "large" {
		return Large
	}
	return Small
}

// Table is one seeded table's manifest entry: its name, row count, and a
// content fingerprint. Two runs of the same Spec produce the same Checksum.
// Error is non-empty when a verifier detects a mismatch; Checksum holds only
// the content fingerprint and must not be used for diagnostic messages.
type Table struct {
	Name     string
	Rows     int64
	Checksum string
	Error    string
}

// Manifest is the result of a seed operation.
type Manifest struct {
	Spec   Spec
	Tables []Table
}

// TableName is the deterministic name of the i-th seeded table.
func TableName(i int) string { return fmt.Sprintf("seed_%02d", i) }

// SeederFunc seeds a backing service identified by dsn and returns a Manifest
// describing what was written. The scheme of dsn selects the backend
// (e.g. "postgres://…", "redis://…"). Each Scenario registers the seeders it
// supports; the gx CLI dispatches to the right one based on --target.
type SeederFunc func(ctx context.Context, dsn string) (Manifest, error)

// DropFunc removes all seeded data from the backing service identified by dsn.
type DropFunc func(ctx context.Context, dsn string) error

// VerifyFunc checks the backing service and returns a Manifest of what it
// finds; the caller can compare against the original Manifest to detect drift.
type VerifyFunc func(ctx context.Context, dsn string) (Manifest, error)

// Progress context key and helpers.
type ctxProgressKey struct{}

// WithProgress attaches an io.Writer to ctx; seed drivers will write per-table
// progress lines to it as they complete.
func WithProgress(ctx context.Context, w io.Writer) context.Context {
	return context.WithValue(ctx, ctxProgressKey{}, w)
}

// Progressf formats and writes a progress message to the writer attached by
// WithProgress. If no writer is attached it is a no-op.
func Progressf(ctx context.Context, format string, args ...any) {
	if w, ok := ctx.Value(ctxProgressKey{}).(io.Writer); ok && w != nil {
		fmt.Fprintf(w, format, args...)
	}
}
