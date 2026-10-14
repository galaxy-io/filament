// Package motherduck implements the MotherDuck sink over DuckDB's native
// driver.
package motherduck

import (
	"context"
	"database/sql/driver"
	"fmt"
	"sort"
	"strings"
	"sync"
	"sync/atomic"
	"unicode"

	duckdb "github.com/marcboeker/go-duckdb/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/motherduck/internal/connection"
)

const (
	sinkName      = "motherduck"
	defaultSchema = "main"
)

type pooledConnection struct {
	conn      driver.Conn
	appenders map[string]*duckdb.Appender
}

func (p *pooledConnection) exec(ctx context.Context, query string) error {
	_, err := p.conn.(driver.ExecerContext).ExecContext(ctx, query, nil)
	return err
}

// Sink writes Arrow batches through DuckDB's appender. Each pooled connection
// is borrowed by one goroutine at a time because DuckDB connections are not
// safe for concurrent use.
type Sink struct {
	// resolve turns connection config into a database to open. The tests
	// point it at a local file to exercise the engine without a token.
	resolve func(filament.Config) (connection.Resolved, error)

	db          *duckdb.Connector
	pool        chan *pooledConnection
	connections []*pooledConnection
	database    string
	schema      string
	run         filament.RunID
	policies    map[string]filament.WritePolicy
	written     atomic.Int64

	tablesMu sync.RWMutex
	tables   map[string]*table
	schemaMu sync.Mutex
}

// New returns an unconfigured MotherDuck sink.
func New() *Sink { return &Sink{resolve: connection.Resolve} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (s *Sink) Name() string { return sinkName }

// Validate checks connection syntax without opening a database.
func (s *Sink) Validate(cfg filament.Config) error {
	if _, err := s.resolve(cfg); err != nil {
		return fmt.Errorf("%s sink: connection config: %w", s.Name(), err)
	}
	return nil
}

// TestConnection opens a short-lived database and executes a trivial query.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := s.resolve(cfg)
	if err != nil {
		return fmt.Errorf("%s sink: connection config: %w", s.Name(), err)
	}
	if err := connection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("%s sink: %w", s.Name(), err)
	}
	return nil
}

// Open opens the database and creates one connection per snapshot worker.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	if s.db != nil {
		return fmt.Errorf("%s sink: already open", s.Name())
	}
	cfg := filament.NewConfig(run.Sink.Config)
	resolved, err := s.resolve(cfg)
	if err != nil {
		return fmt.Errorf("%s sink: connection config: %w", s.Name(), err)
	}
	schema := strings.TrimSpace(cfg.String("schema"))
	if schema == "" {
		schema = defaultSchema
	}
	database := strings.TrimSpace(cfg.String("database"))

	db, err := connection.Open(resolved)
	if err != nil {
		return fmt.Errorf("%s sink: open: %w", s.Name(), err)
	}
	parallelism := max(1, run.Options.SnapshotParallelism)
	connections := make([]*pooledConnection, 0, parallelism)
	cleanup := func() {
		for _, pooled := range connections {
			_ = pooled.conn.Close()
		}
		_ = db.Close()
	}
	schemaName := qualifySchema(database, schema)
	for range parallelism {
		conn, err := db.Connect(ctx)
		if err != nil {
			cleanup()
			return fmt.Errorf("%s sink: connect: %w", s.Name(), err)
		}
		pooled := &pooledConnection{conn: conn, appenders: make(map[string]*duckdb.Appender)}
		connections = append(connections, pooled)
		if err := pooled.exec(ctx, "CREATE SCHEMA IF NOT EXISTS "+schemaName); err != nil {
			cleanup()
			return fmt.Errorf("%s sink: create schema %q: %w", s.Name(), schema, err)
		}
	}

	pool := make(chan *pooledConnection, len(connections))
	for _, conn := range connections {
		pool <- conn
	}
	s.db = db
	s.pool = pool
	s.connections = connections
	s.database = database
	s.schema = schema
	s.run = run.Run
	s.policies = run.WritePolicies
	s.written.Store(0)
	s.tablesMu.Lock()
	s.tables = make(map[string]*table)
	s.tablesMu.Unlock()
	return nil
}

// Commit closes all appenders, promotes staged tables transactionally, and
// releases the database.
func (s *Sink) Commit(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	defer s.release()
	if err := s.closeAppenders(); err != nil {
		return fmt.Errorf("%s sink: close appenders: %w", s.Name(), err)
	}
	conn, err := s.take(ctx)
	if err != nil {
		return err
	}
	defer s.put(conn)
	for _, tbl := range s.tableSnapshot() {
		if tbl.stage == "" {
			continue
		}
		if err := s.commitTable(ctx, conn, tbl); err != nil {
			return err
		}
	}
	return nil
}

func (s *Sink) commitTable(ctx context.Context, conn *pooledConnection, tbl *table) error {
	if err := conn.exec(ctx, "BEGIN TRANSACTION"); err != nil {
		return fmt.Errorf("%s sink: begin commit for %q: %w", s.Name(), tbl.name, err)
	}
	rollback := func() { _ = conn.exec(context.WithoutCancel(ctx), "ROLLBACK") }
	var statements []string
	switch tbl.mode {
	case filament.WriteReplace:
		statements = []string{
			"DROP TABLE IF EXISTS " + tbl.qualified,
			"ALTER TABLE " + tbl.writeTo + " RENAME TO " + quoteIdent(tbl.name),
		}
	case filament.WriteUpsert:
		statements = []string{tbl.mergeSQL, "DROP TABLE " + tbl.writeTo}
	default:
		rollback()
		return fmt.Errorf("%s sink: cannot commit staged mode %q for %q", s.Name(), tbl.mode, tbl.name)
	}
	for _, statement := range statements {
		if err := conn.exec(ctx, statement); err != nil {
			rollback()
			return fmt.Errorf("%s sink: commit %q: %w", s.Name(), tbl.name, err)
		}
	}
	if err := conn.exec(ctx, "COMMIT"); err != nil {
		rollback()
		return fmt.Errorf("%s sink: commit transaction for %q: %w", s.Name(), tbl.name, err)
	}
	return nil
}

// Abort closes appenders, drops run-scoped stages, and releases the database.
func (s *Sink) Abort(ctx context.Context) error {
	if s.db == nil {
		return nil
	}
	defer s.release()
	var first error
	if err := s.closeAppenders(); err != nil {
		first = fmt.Errorf("%s sink: close appenders: %w", s.Name(), err)
	}
	cleanup := context.WithoutCancel(ctx)
	conn, err := s.take(cleanup)
	if err != nil {
		if first == nil {
			first = err
		}
		return first
	}
	defer s.put(conn)
	for _, tbl := range s.tableSnapshot() {
		if tbl.stage == "" {
			continue
		}
		if err := conn.exec(cleanup, "DROP TABLE IF EXISTS "+tbl.writeTo); err != nil && first == nil {
			first = fmt.Errorf("%s sink: drop stage for %q: %w", s.Name(), tbl.name, err)
		}
	}
	return first
}

func (s *Sink) take(ctx context.Context) (*pooledConnection, error) {
	if s.pool == nil {
		return nil, fmt.Errorf("%s sink: database is not open", s.Name())
	}
	select {
	case conn := <-s.pool:
		return conn, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *Sink) put(conn *pooledConnection) {
	if s.pool != nil && conn != nil {
		s.pool <- conn
	}
}

func (s *Sink) closeAppenders() error {
	var first error
	for _, conn := range s.connections {
		resources := make([]string, 0, len(conn.appenders))
		for resource := range conn.appenders {
			resources = append(resources, resource)
		}
		sort.Strings(resources)
		for _, resource := range resources {
			if err := conn.appenders[resource].Close(); err != nil && first == nil {
				first = fmt.Errorf("%s: %w", resource, err)
			}
			delete(conn.appenders, resource)
		}
	}
	return first
}

func (s *Sink) release() {
	_ = s.closeAppenders()
	for _, conn := range s.connections {
		_ = conn.conn.Close()
	}
	if s.db != nil {
		_ = s.db.Close()
	}
	s.db = nil
	s.pool = nil
	s.connections = nil
	s.database = ""
	s.schema = ""
	s.run = ""
	s.policies = nil
	s.tablesMu.Lock()
	s.tables = nil
	s.tablesMu.Unlock()
}

func (s *Sink) modeFor(resource string) filament.WriteMode {
	policy, ok := s.policies[resource]
	if !ok {
		policy = s.policies[""]
	}
	return policy.Capability.Mode
}

func (s *Sink) policyFor(resource string) filament.WritePolicy {
	policy, ok := s.policies[resource]
	if !ok {
		policy = s.policies[""]
	}
	return policy
}

func (s *Sink) tableSnapshot() []*table {
	s.tablesMu.RLock()
	resources := make([]string, 0, len(s.tables))
	for resource := range s.tables {
		resources = append(resources, resource)
	}
	sort.Strings(resources)
	tables := make([]*table, 0, len(resources))
	for _, resource := range resources {
		tables = append(tables, s.tables[resource])
	}
	s.tablesMu.RUnlock()
	return tables
}

func stageTableName(table string, run filament.RunID) string {
	runes := make([]rune, 0, len(run))
	for _, r := range string(run) {
		if unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_' {
			runes = append(runes, r)
		} else {
			runes = append(runes, '_')
		}
	}
	if len(runes) == 0 {
		runes = append(runes, []rune("run")...)
	}
	return table + "__stage_" + string(runes)
}
