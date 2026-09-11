// Package bigquery implements the Google BigQuery sink.
package bigquery

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"cloud.google.com/go/bigquery"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	bigqueryconnection "github.com/galaxy-io/filament/connectors/bigquery/internal/connection"
)

const sinkName = "bigquery"

var errWritesUnavailable = errors.New("bigquery sink: writes are not implemented")

// Sink owns the BigQuery client and materializes typed destination tables.
// Row loading is added in a subsequent implementation tranche.
type Sink struct {
	client   *bigquery.Client
	project  string
	dataset  string
	location string
}

// New returns an unconfigured BigQuery sink.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
	_ filament.Schematized       = (*Sink)(nil)
)

// Name identifies this sink implementation.
func (*Sink) Name() string { return sinkName }

// Validate checks connection syntax without accessing the network.
func (*Sink) Validate(cfg filament.Config) error {
	if _, err := bigqueryconnection.Resolve(cfg); err != nil {
		return fmt.Errorf("bigquery sink: connection config: %w", err)
	}
	return nil
}

// TestConnection authenticates with BigQuery and executes a trivial query.
func (*Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := bigqueryconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("bigquery sink: connection config: %w", err)
	}
	if err := bigqueryconnection.Test(ctx, resolved); err != nil {
		return fmt.Errorf("bigquery sink: %w", err)
	}
	return nil
}

// Open establishes the run's BigQuery client and ensures its destination
// dataset exists. Resource tables are created later by EnsureSchema.
func (s *Sink) Open(ctx context.Context, run filament.RunSpec) error {
	if s.client != nil {
		return fmt.Errorf("bigquery sink: already open")
	}
	cfg := filament.NewConfig(run.Sink.Config)
	resolved, err := bigqueryconnection.Resolve(cfg)
	if err != nil {
		return fmt.Errorf("bigquery sink: connection config: %w", err)
	}
	dataset := strings.TrimSpace(cfg.String("dataset"))
	ddl, err := createDatasetDDL(resolved.ProjectID, dataset, resolved.Location)
	if err != nil {
		return fmt.Errorf("bigquery sink: dataset: %w", err)
	}
	client, err := bigqueryconnection.Open(ctx, resolved)
	if err != nil {
		return fmt.Errorf("bigquery sink: open: %w", err)
	}
	s.client = client
	s.project = resolved.ProjectID
	s.dataset = dataset
	s.location = resolved.Location
	if err := s.execute(ctx, ddl); err != nil {
		s.release()
		return fmt.Errorf("bigquery sink: create dataset %s: %w", qualified(s.project, s.dataset), err)
	}
	return nil
}

// Apply is unavailable until the BigQuery write implementation is added.
func (*Sink) Apply(context.Context, *arrowbatch.Batch, filament.ApplyOptions) (filament.WriteReceipt, error) {
	return filament.WriteReceipt{}, errWritesUnavailable
}

// Commit closes the client. BigQuery schema DDL is committed when each job
// completes.
func (s *Sink) Commit(context.Context) error {
	s.release()
	return nil
}

// Abort closes the client. Additive schema changes are intentionally retained.
func (s *Sink) Abort(context.Context) error {
	s.release()
	return nil
}

func (s *Sink) execute(ctx context.Context, statement string) error {
	query := s.client.Query(statement)
	if s.location != "" {
		query.Location = s.location
	}
	job, err := query.Run(ctx)
	if err != nil {
		return err
	}
	status, err := job.Wait(ctx)
	if err != nil {
		return err
	}
	return status.Err()
}

func (s *Sink) release() {
	if s.client != nil {
		_ = s.client.Close()
		s.client = nil
	}
}
