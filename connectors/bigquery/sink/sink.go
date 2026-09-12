// Package bigquery implements the Google BigQuery sink.
package bigquery

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	bigqueryconnection "github.com/galaxy-io/filament/connectors/bigquery/internal/connection"
)

const sinkName = "bigquery"

var errWritesUnavailable = errors.New("bigquery sink: writes are not implemented")

// Sink exposes BigQuery connection configuration and connectivity checks.
// Schema and write capabilities are added in subsequent implementation tranches.
type Sink struct{}

// New returns an unconfigured BigQuery sink.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
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

// Open is unavailable until the BigQuery write implementation is added.
func (*Sink) Open(context.Context, filament.RunSpec) error { return errWritesUnavailable }

// Apply is unavailable until the BigQuery write implementation is added.
func (*Sink) Apply(context.Context, *arrowbatch.Batch, filament.ApplyOptions) (filament.WriteReceipt, error) {
	return filament.WriteReceipt{}, errWritesUnavailable
}

// Commit is unavailable until the BigQuery write implementation is added.
func (*Sink) Commit(context.Context) error { return errWritesUnavailable }

// Abort is a no-op because this tranche never opens a writable sink run.
func (*Sink) Abort(context.Context) error { return nil }
