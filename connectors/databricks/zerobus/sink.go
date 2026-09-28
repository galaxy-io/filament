// Package zerobus implements a Databricks sink that streams records into Delta
// tables through the Zerobus Ingest API over Arrow Flight. It is append-only:
// each Arrow batch is serialized to Arrow IPC bytes and ingested into a
// pre-existing Delta table whose schema matches the source resource.
package zerobus

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/apache/arrow-go/v18/arrow"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/connectors/databricks/internal/connection"
)

const sinkName = "databrickszerobus"

// ingestStream is one open Zerobus Arrow Flight stream for a single table.
type ingestStream interface {
	Ingest(ipc []byte) (offset int64, err error)
	Flush() error
	Close() error
}

// ingestClient opens per-table streams. It isolates the CGO-backed Zerobus SDK
// (see sdk.go) so the sink is unit-testable with a fake.
type ingestClient interface {
	OpenStream(ctx context.Context, table string, schemaIPC []byte) (ingestStream, error)
	Close() error
}

// clientFactory builds an ingestClient from resolved connection settings and
// the configured IPC compression codec.
type clientFactory func(r connection.Resolved, compression string) (ingestClient, error)

// Sink is the Databricks Zerobus sink. One stream is opened per resource in
// EnsureSchema; Apply ingests batches; Commit flushes and closes all streams.
type Sink struct {
	newClient   clientFactory
	newExecutor executorFactory

	client      ingestClient
	exec        tableCreator
	resolved    connection.Resolved
	catalog     string
	schema      string
	create      bool
	compression string
	run         filament.RunID
	policies    map[string]filament.WritePolicy
	streams     map[string]ingestStream
	schemas     map[string]*arrow.Schema
	tel         *telemetry
}

// New returns an unconfigured sink backed by the official Zerobus SDK and the
// Databricks Statement Execution API for optional table creation. Open
// establishes the client.
func New() *Sink { return &Sink{newClient: newSDKClient, newExecutor: newStatementExecutor} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (*Sink) Name() string { return sinkName }

// Validate checks connection syntax without opening a network connection.
func (s *Sink) Validate(cfg filament.Config) error {
	if _, err := connection.Resolve(cfg); err != nil {
		return fmt.Errorf("databrickszerobus sink: connection config: %w", err)
	}
	if err := validateCompression(strings.TrimSpace(cfg.String("ipc_compression"))); err != nil {
		return fmt.Errorf("databrickszerobus sink: %w", err)
	}
	return nil
}

// TestConnection resolves the config and constructs a Zerobus SDK handle to
// surface malformed endpoints early. Full credential and table authorization
// are verified when a stream opens, since Zerobus binds the table name into the
// authorization at stream creation.
func (s *Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := connection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("databrickszerobus sink: connection config: %w", err)
	}
	client, err := s.factory()(resolved, strings.TrimSpace(cfg.String("ipc_compression")))
	if err != nil {
		return fmt.Errorf("databrickszerobus sink: %w", err)
	}
	return client.Close()
}

// factory returns the client factory, preferring an injected one (tests) and
// falling back to the real SDK client.
func (s *Sink) factory() clientFactory {
	if s.newClient != nil {
		return s.newClient
	}
	return newSDKClient
}

// Open resolves configuration and constructs the Zerobus client for the run.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	cfg := filament.NewConfig(run.Sink.Config)
	resolved, err := connection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("databrickszerobus sink: connection config: %w", err)
	}
	catalog := strings.TrimSpace(cfg.String("catalog"))
	if catalog == "" {
		return fmt.Errorf("databrickszerobus sink: catalog is required")
	}
	schema := strings.TrimSpace(cfg.String("schema"))
	if schema == "" {
		return fmt.Errorf("databrickszerobus sink: schema is required")
	}
	compression := strings.TrimSpace(cfg.String("ipc_compression"))
	if err := validateCompression(compression); err != nil {
		return fmt.Errorf("databrickszerobus sink: %w", err)
	}
	client, err := s.factory()(resolved, compression)
	if err != nil {
		return fmt.Errorf("databrickszerobus sink: open: %w", err)
	}

	s.client = client
	s.resolved = resolved
	s.catalog = catalog
	s.schema = schema
	s.create = cfg.Bool("create_table")
	s.compression = compression
	s.run = run.Run
	s.policies = run.WritePolicies
	s.streams = map[string]ingestStream{}
	s.schemas = map[string]*arrow.Schema{}
	s.tel = newTelemetry()
	return nil
}

// Commit flushes every open stream to durability, then closes them.
func (s *Sink) Commit(ctx context.Context) error {
	defer s.release()
	if s.client == nil {
		return nil
	}
	var first error
	for resource, stream := range s.streams {
		start := time.Now()
		if err := stream.Flush(); err != nil {
			s.tel.recordError(ctx, resource, retryable(err))
			if first == nil {
				first = fmt.Errorf("databrickszerobus sink: flush %q: %w", resource, err)
			}
			// Free the handle explicitly: the SDK's Free() does not close
			// outstanding streams, so a skipped Close leaks it until GC.
			_ = stream.Close()
			continue
		}
		s.tel.recordFlush(ctx, resource, time.Since(start))
		if err := stream.Close(); err != nil && first == nil {
			first = fmt.Errorf("databrickszerobus sink: close stream %q: %w", resource, err)
		}
	}
	return first
}

// Abort closes streams without waiting for a durable flush. Zerobus delivers
// at-least-once, so batches already acknowledged remain in the table.
func (s *Sink) Abort(ctx context.Context) error {
	defer s.release()
	if s.client == nil {
		return nil
	}
	for _, stream := range s.streams {
		_ = stream.Close()
	}
	return nil
}

// release drops stream references and closes the client and executor. It runs
// on both Commit and Abort so the sink holds no open handles afterward.
func (s *Sink) release() {
	for k := range s.streams {
		delete(s.streams, k)
	}
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
	if s.exec != nil {
		_ = s.exec.Close()
		s.exec = nil
	}
}

// execFactory returns the table-creator factory, preferring an injected one
// (tests) and falling back to the Statement Execution API executor.
func (s *Sink) execFactory() executorFactory {
	if s.newExecutor != nil {
		return s.newExecutor
	}
	return newStatementExecutor
}
