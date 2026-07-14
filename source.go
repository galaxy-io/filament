package ingestion

import "context"

// Source is the connector contract for reading data: describe itself,
// validate and take config, extract into a RecordSink, and tear down.
type Source interface {
	Spec() ConnectorSpec
	Validate(cfg Config) error
	Configure(ctx context.Context, cfg Config) error
	Extract(ctx context.Context, sink RecordSink, opts ExtractOpts) error
	Teardown(ctx context.Context) error
}

// Resumable is the optional source contract for checkpointed extraction. prev maps
// each resource to the cursor to resume from (a plan produced by ResumePlanner,
// carrying any progress from a prior run); a resource absent from prev — or mapped
// to nil — is read from the start. ExtractFrom stamps each Record with its shard
// (Record.Part) and keyset position (Record.Key) so the pipeline can advance and
// persist the cursor.
type Resumable interface {
	ExtractFrom(ctx context.Context, sink RecordSink, opts ExtractOpts, prev map[string]Checkpoint) error
}

// ChangeSource is the optional contract for CDC extraction: emit ordered
// changes from per-resource checkpoints instead of a full read.
type ChangeSource interface {
	ExtractChanges(ctx context.Context, sink RecordSink, opts ChangeExtractOpts) error
}

// ResourcePlanner lets a source translate externally selected resources into the
// concrete resource names it will emit. Selectors remain source-private; the
// returned names are used by the engine for policy, schema, and checkpoint setup.
type ResourcePlanner interface {
	PlanResources(ctx context.Context, resources, selectors []string) ([]string, error)
}

// ResumePlanner builds the initial per-resource checkpoint plan for a resumable run:
// the shard layout (PK split) plus any cursor carried over from prev. The engine
// persists the returned plan before extraction so a later resume sees a complete,
// stable shard layout. A resource that cannot be resumed (no primary key) maps to
// nil — the source reads it non-resumably and it is re-read whole on resume.
type ResumePlanner interface {
	PlanResume(ctx context.Context, resources []string, prev map[string]Checkpoint) (map[string]Checkpoint, error)
}

// Discoverable is the optional contract for browsing a source's available
// resources.
type Discoverable interface {
	Discover(ctx context.Context, opts DiscoverOpts) (DiscoverResult, error)
}

// RateLimited lets a source declare its own extraction rate ceiling.
type RateLimited interface {
	Limits() RatePolicy
}

// LiveValidatable is the optional contract for probing connectivity with a
// config before any run uses it.
type LiveValidatable interface {
	TestConnection(ctx context.Context, cfg Config) error
}

// ConnectorSpec is a source's self-description: identity, supported modes and
// policies, config schema, and resource capabilities. It powers the catalog.
type ConnectorSpec struct {
	Name           string
	DisplayName    string
	Version        string
	Modes          []ReplicationMode
	SourcePolicies []SourcePolicy
	Config         ConfigSchema
	Resources      ResourceCapabilities
}

// ConfigSchema declares the fields a connector's Config accepts.
type ConfigSchema struct{ Fields []ConfigField }

// ConfigField describes one config field for validation and UI rendering.
type ConfigField struct {
	Name     string
	Type     FieldType
	Required bool
	Default  any
	Enum     []string
	Help     string
	Scope    FieldScope
}

// FieldType is a ConfigField's value kind.
type FieldType int

// The config field types.
const (
	FieldString FieldType = iota
	FieldInt
	FieldBool
	FieldSecret // rendered masked; stored as a ref
	FieldDuration
	FieldEnum
	FieldObject
)

// FieldScope is where a config field is set: on the reusable connection or per
// pipeline node.
type FieldScope int

// The field scopes.
const (
	ScopeUnspecified FieldScope = iota
	ScopeConnection
	ScopePipeline
)

// IsPipeline reports whether the field is set per pipeline node
// rather than on the reusable connection.
func (s FieldScope) IsPipeline() bool { return s == ScopePipeline }

// ReplicationMode is how a source reads: full scan, incremental from a
// cursor, or CDC.
type ReplicationMode int

// The replication modes.
const (
	ModeFull ReplicationMode = iota
	ModeIncremental
	ModeCDC
)

// ResourceCapabilities advertises what a source can do per resource.
type ResourceCapabilities struct{ Discoverable, PerResourceCursor bool }

// ExtractOpts scopes one extraction: which resources, in what mode, how fast.
type ExtractOpts struct {
	Resources   []string
	Selectors   []string
	Mode        ReplicationMode
	Limit       int // 0 = unbounded
	Parallelism int
}

// ChangeExtractOpts scopes one CDC extraction: resources plus the checkpoints
// to resume each from.
type ChangeExtractOpts struct {
	Resources   []string
	Checkpoints map[string]Checkpoint
	Limit       int
}

// DiscoverOpts controls discovery; Refresh bypasses any cached catalog.
type DiscoverOpts struct{ Refresh bool }

// DiscoverResult is the resource catalog a Discoverable source returns.
type DiscoverResult struct{ Resources []Resource }

// Resource is one discoverable unit of data (table, stream, endpoint) and the
// metadata needed to select and plan it.
type Resource struct {
	Name        string
	Selector    string
	Selectable  bool
	PrimaryKey  []string
	Schema      *RecordSchema // optional
	Estimated   int64
	DisplayName string
	Metadata    map[string]string
}

// RatePolicy is a token-bucket rate limit on source requests.
type RatePolicy struct {
	RequestsPerSecond float64
	Burst             int
}
