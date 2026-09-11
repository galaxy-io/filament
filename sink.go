package filament

import (
	"context"

	"github.com/galaxy-io/filament/arrowbatch"
)

// Sink is the connector contract for writing data: open for a run, apply
// batches under a write policy, then commit or abort. Apply borrows b for the
// duration of the call; a sink that keeps it afterward must Retain it and
// eventually Release that reference.
type Sink interface {
	Spec() SinkSpec // identity + config schema + capabilities (powers the catalog)
	Open(ctx context.Context, run RunSpec) error
	Apply(ctx context.Context, b *arrowbatch.Batch, opts ApplyOptions) (WriteReceipt, error)
	Commit(ctx context.Context) error
	Abort(ctx context.Context) error
	Name() string
}

// ApplyOptions carries the per-resource write policy governing one Apply.
type ApplyOptions struct {
	Policy WritePolicy
}

// Transactional is the optional sink contract for staged, atomic runs: write
// into a stage, promote on success.
type Transactional interface {
	Stage(ctx context.Context) (StageID, error)
	Promote(ctx context.Context, id StageID) error
}

// Upsertable is the optional sink contract for key-based merge writes.
type Upsertable interface {
	Upsert(ctx context.Context, b *arrowbatch.Batch, keys []string) (WriteReceipt, error)
}

// Schematized is the optional sink contract for typed DDL: materialize a
// resource's schema before its records arrive.
type Schematized interface {
	EnsureSchema(ctx context.Context, resource string, schema RecordSchema) error
}

// SinkSpec is a sink's self-description: identity, config schema, and
// capabilities. It powers the catalog.
type SinkSpec struct {
	Name         string
	DisplayName  string
	Description  string
	DarkLogoURL  string
	LightLogoURL string
	Version      string
	Maturity     ConnectorMaturity
	Config       ConfigSchema
	Capabilities SinkCapabilities

	// SchemaField names the pipeline-scoped config field holding where the
	// sink lands this pipeline's output — a schema, namespace, database, or
	// key prefix — so the server can default it from the source connection's
	// normalized name. Empty for sinks without one.
	SchemaField string
}

// SinkCapabilities advertises the optional contracts and write modes a sink
// supports, so the engine can match it to an ingestion type.
type SinkCapabilities struct {
	Transactional bool
	Upsertable    bool
	Schematized   bool
	// EncodedIntegrity requires Apply to verify the final serialized bytes at
	// its write boundary and return the resulting EncodedCRC as evidence.
	EncodedIntegrity bool
	WritePolicies    []WritePolicyCapability
	// PreferredBatchRows is the sink's preferred rows per Apply, used when the
	// run does not set Options.BatchMaxRows.
	// 0 defers to the engine default.
	PreferredBatchRows int
	// PreferredBatchBytes is the sink's preferred approximate unencoded value
	// bytes per Apply, used when the run does not set Options.BatchMaxBytes.
	// 0 leaves batches unbounded by bytes.
	PreferredBatchBytes int64
}
