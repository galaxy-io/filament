// Package snowflake implements the Snowflake sink.
package snowflake

import (
	"context"
	"errors"
	"fmt"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
)

const sinkName = "snowflake"

var errWritesUnavailable = errors.New("snowflake sink: writes are not implemented")

// Sink exposes Snowflake connection configuration and connectivity checks.
// Write capabilities are added in subsequent implementation tranches.
type Sink struct{}

// New returns an unconfigured Snowflake sink.
func New() *Sink { return &Sink{} }

var (
	_ filament.Sink              = (*Sink)(nil)
	_ filament.ConfigValidatable = (*Sink)(nil)
	_ filament.LiveValidatable   = (*Sink)(nil)
)

// Spec describes the sink's connection configuration. It intentionally
// advertises no write policies until the snapshot implementation is available.
func (*Sink) Spec() filament.SinkSpec {
	return filament.SinkSpec{
		Name:         sinkName,
		DisplayName:  "Snowflake",
		Description:  "Cloud-native data platform for scalable analytics, elastic compute, and secure data sharing.",
		DarkLogoURL:  "https://cdn.getgalaxy.io/sources/source-icon-snowflake-dark.svg",
		LightLogoURL: "https://cdn.getgalaxy.io/sources/source-icon-snowflake-light.svg",
		Version:      "1",
		Config: filament.ConfigSchema{Fields: append(connectionFields(), filament.ConfigField{
			Name: "schema", Type: filament.FieldString, Default: defaultSchema,
			Scope: filament.ScopePipeline, Help: "Destination schema. Empty defaults to the normalized source connection name.",
		})},
		SchemaField: "schema",
	}
}

// Name identifies this sink implementation.
func (*Sink) Name() string { return sinkName }

// Validate checks connection syntax and credentials without accessing the network.
func (*Sink) Validate(cfg filament.Config) error {
	if _, err := resolveConnection(cfg); err != nil {
		return fmt.Errorf("snowflake sink: connection config: %w", err)
	}
	return nil
}

// TestConnection authenticates with Snowflake using a short-lived database handle.
func (*Sink) TestConnection(ctx context.Context, cfg filament.Config) error {
	resolved, err := resolveConnection(cfg)
	if err != nil {
		return fmt.Errorf("snowflake sink: connection config: %w", err)
	}
	db := resolved.open()
	defer func() { _ = db.Close() }()
	if err := db.PingContext(ctx); err != nil {
		return fmt.Errorf("snowflake sink: ping: %w", err)
	}
	return nil
}

// Open is unavailable until the Snowflake write implementation is added.
func (*Sink) Open(context.Context, filament.RunSpec) error { return errWritesUnavailable }

// Apply is unavailable until the Snowflake write implementation is added.
func (*Sink) Apply(context.Context, *arrowbatch.Batch, filament.ApplyOptions) (filament.WriteReceipt, error) {
	return filament.WriteReceipt{}, errWritesUnavailable
}

// Commit is unavailable until the Snowflake write implementation is added.
func (*Sink) Commit(context.Context) error { return errWritesUnavailable }

// Abort is a no-op because this tranche never opens a writable sink run.
func (*Sink) Abort(context.Context) error { return nil }
