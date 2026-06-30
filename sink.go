package ingestion

import "context"

type Sink interface {
	Spec() SinkSpec // identity + config schema + capabilities (powers the catalog)
	Open(ctx context.Context, run RunSpec) error
	Apply(ctx context.Context, b Batch, opts ApplyOptions) (WriteReceipt, error)
	Commit(ctx context.Context) error
	Abort(ctx context.Context) error
	Name() string
}

type ApplyOptions struct {
	Policy WritePolicy
}

type Transactional interface {
	Stage(ctx context.Context) (StageID, error)
	Promote(ctx context.Context, id StageID) error
}

type Upsertable interface {
	Upsert(ctx context.Context, b Batch, keys []string) (WriteReceipt, error)
}

type Schematized interface {
	EnsureSchema(ctx context.Context, resource string, schema RecordSchema) error
}

type SinkSpec struct {
	Name         string
	DisplayName  string
	Version      string
	Config       ConfigSchema
	Capabilities SinkCapabilities
}

type SinkCapabilities struct {
	Transactional bool
	Upsertable    bool
	Schematized   bool
	WritePolicies []WritePolicyCapability
}
