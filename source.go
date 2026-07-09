package ingestion

import "context"

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

type Discoverable interface {
	Discover(ctx context.Context, opts DiscoverOpts) (DiscoverResult, error)
}

type RateLimited interface {
	Limits() RatePolicy
}

type LiveValidatable interface {
	TestConnection(ctx context.Context, cfg Config) error
}

type ConnectorSpec struct {
	Name           string
	DisplayName    string
	Version        string
	Modes          []ReplicationMode
	SourcePolicies []SourcePolicy
	Config         ConfigSchema
	Resources      ResourceCapabilities
}

type ConfigSchema struct{ Fields []ConfigField }

type ConfigField struct {
	Name     string
	Type     FieldType
	Required bool
	Default  any
	Enum     []string
	Help     string
	Scope FieldScope
}

type FieldType int

const (
	FieldString FieldType = iota
	FieldInt
	FieldBool
	FieldSecret // rendered masked; stored as a ref
	FieldDuration
	FieldEnum
	FieldObject
)

type FieldScope int

const (
	ScopeUnspecified FieldScope = iota 
	ScopeConnection
	ScopePipeline
)

// IsPipeline reports whether the field is set per pipeline node
// rather than on the reusable connection.
func (s FieldScope) IsPipeline() bool { return s == ScopePipeline }

type ReplicationMode int

const (
	ModeFull ReplicationMode = iota
	ModeIncremental
	ModeCDC
)

type ResourceCapabilities struct{ Discoverable, PerResourceCursor bool }

type ExtractOpts struct {
	Resources   []string
	Selectors   []string
	Mode        ReplicationMode
	Limit       int // 0 = unbounded
	Parallelism int
}

type ChangeExtractOpts struct {
	Resources   []string
	Checkpoints map[string]Checkpoint
	Limit       int
}

type DiscoverOpts struct{ Refresh bool }

type DiscoverResult struct{ Resources []Resource }

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

type RatePolicy struct {
	RequestsPerSecond float64
	Burst             int
}
