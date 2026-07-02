package postgres

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	ingestion "github.com/galaxy-io/filament"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver used by NewSQLDB/Migrate
)

// DefaultTenantID is the tenant row seeded for POC runs when none is configured.
const DefaultTenantID = "t1"

// errNotImplemented marks datastore behavior that is not yet written. The
// connection + migration plumbing (NewSQLDB, NewPool, Migrate) is real; the
// DataStore read/write methods are stubs so the control-plane and worker
// binaries compile and can be built into images while the Postgres datastore is
// finished. See TODO(datastore) on Store.
var errNotImplemented = errors.New("datastore/postgres: not implemented")

// NewSQLDB opens a database/sql handle via the pgx stdlib driver, suitable for
// goose migrations (see Migrate).
func NewSQLDB(dsn string) (*sql.DB, error) {
	db, err := sql.Open("pgx", dsn)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: open sql db: %w", err)
	}
	return db, nil
}

// NewPool opens a pgx connection pool for the datastore. The pool connects
// lazily on first use.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		return nil, fmt.Errorf("datastore/postgres: new pool: %w", err)
	}
	return pool, nil
}

// Store is the Postgres-backed ingestion.DataStore.
//
// TODO (@atterpac to implement)
type Store struct {
	pool *pgxpool.Pool
}

// New returns a Store backed by the given pool.
func New(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

var _ ingestion.DataStore = (*Store)(nil)

func (s *Store) Name() string { return "postgres" }

// EnsureTenant upserts a tenant row. TODO(datastore).
func (s *Store) EnsureTenant(ctx context.Context, id ingestion.TenantID, name string) error {
	return errNotImplemented
}

func (s *Store) SaveRun(ctx context.Context, r ingestion.RunState) error {
	return errNotImplemented
}

func (s *Store) LoadRun(ctx context.Context, id ingestion.RunID) (ingestion.RunState, error) {
	return ingestion.RunState{}, errNotImplemented
}

func (s *Store) ListRuns(ctx context.Context, f ingestion.RunFilter) ([]ingestion.RunState, error) {
	return nil, errNotImplemented
}

func (s *Store) UpsertResource(ctx context.Context, rs ingestion.ResourceState) error {
	return errNotImplemented
}

func (s *Store) ListResources(ctx context.Context, id ingestion.RunID) ([]ingestion.ResourceState, error) {
	return nil, errNotImplemented
}

func (s *Store) SaveCheckpoint(ctx context.Context, id ingestion.RunID, cp ingestion.Checkpoint) error {
	return errNotImplemented
}

func (s *Store) LoadCheckpoint(ctx context.Context, id ingestion.RunID, resource string) (ingestion.Checkpoint, error) {
	return nil, errNotImplemented
}

func (s *Store) DedupSeen(ctx context.Context, tenant string, run ingestion.RunID, seq uint64) (bool, error) {
	return false, errNotImplemented
}
