package filament

import (
	"context"
	"time"

	"github.com/galaxy-io/filament/arrowbatch"
)

// Sink owns a destination session independently of source execution mode.
// Open acquires session resources. Begin starts a finite write. Apply borrows
// its batch only for the call. Commit establishes the advertised durability for
// every successful Apply. Abort cleans up unfinished writes according to the
// write policy; it does not promise rollback of already durable effects. Close
// releases session resources and pending in-memory work, including after a
// failed Open. Close reports an error when cleanup cannot establish that
// destination work has stopped. An Apply or Commit error alone is not proof
// of failed cleanup.
//
// The runner serializes lifecycle calls and joins all Apply calls before Commit,
// Abort, or Close. It never retries Apply or an ambiguous Commit in the same
// session. ReusableSession admits repeated Begin/Commit boundaries. A bounded
// run uses one boundary. Source progress and ownership belong to the runner
// and are never passed to the sink.
type Sink interface {
	Spec() SinkSpec // identity + config schema + capabilities (powers the catalog)
	Open(ctx context.Context, run RunSpec) error
	Begin(ctx context.Context) error
	Apply(ctx context.Context, b *arrowbatch.Batch, opts ApplyOptions) (WriteReceipt, error)
	Commit(ctx context.Context) error
	Abort(ctx context.Context) error
	Close(ctx context.Context) error
	Name() string
}

// ApplyOptions carries the per-resource write policy governing one Apply.
type ApplyOptions struct {
	Policy WritePolicy
}

// Schematized is the optional sink contract for typed DDL: materialize a
// resource's schema before its records arrive. The runner supplies destination
// resource names consistently to Open, EnsureSchema, and Apply.
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
	// ReusableSession permits repeated Begin/Commit boundaries on one open sink.
	// Otherwise the runner opens a fresh instance for each finite write.
	ReusableSession bool
	Schematized     bool
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
	// PreferredFlushInterval is used when the pipeline does not explicitly
	// choose a flush interval. Zero uses the engine default.
	PreferredFlushInterval time.Duration
}
