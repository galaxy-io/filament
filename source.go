package filament

import (
	"context"
	"fmt"
	"strings"
	"time"
)

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

// IncrementalPlanner builds a durable per-resource plan from the previous
// cross-run watermark and versioned cursor configuration.
type IncrementalPlanner interface {
	PlanIncremental(ctx context.Context, resources []string, prev map[string]Checkpoint, cursors map[string]ResourceCursorConfig) (map[string]Checkpoint, error)
}

// CursorColumnProvider describes which resource columns can safely serve as
// durable incremental cursors and which one the connector recommends.
type CursorColumnProvider interface {
	CursorColumns(ctx context.Context, resource string) ([]CursorColumn, error)
}

// CursorColumn is a schema field annotated with its incremental-cursor
// capabilities. Rank is connector-specific; lower positive values are better.
type CursorColumn struct {
	SchemaField
	PrimaryKey       bool
	Eligible         bool
	Recommended      bool
	Rank             int
	Configurable     bool
	SupportsLookback bool
	Warning          string
}

// ResolveCursorVersionPolicy binds a resource's configured or recommended
// incremental cursor into the generic version contract used by sinks. A zero
// policy means the source does not expose cursor metadata and callers should
// retain their insertion-order fallback.
func ResolveCursorVersionPolicy(ctx context.Context, src Source, resource string, config ResourceCursorConfig) (VersionPolicy, error) {
	provider, ok := src.(CursorColumnProvider)
	if !ok {
		return VersionPolicy{}, nil
	}
	columns, err := provider.CursorColumns(ctx, resource)
	if err != nil {
		return VersionPolicy{}, fmt.Errorf("cursor columns for %q: %w", resource, err)
	}
	var selected *CursorColumn
	for i := range columns {
		candidate := &columns[i]
		if config.Field != "" {
			if strings.EqualFold(candidate.Name, config.Field) {
				selected = candidate
				break
			}
			continue
		}
		if selected == nil && candidate.Recommended {
			selected = candidate
		}
	}
	if selected == nil {
		if config.Field != "" {
			return VersionPolicy{}, fmt.Errorf("incremental %q cursor column %q does not exist", resource, config.Field)
		}
		return VersionPolicy{}, fmt.Errorf("incremental %q has no recommended cursor column; configure the pipeline resource cursor", resource)
	}
	if !selected.Eligible {
		return VersionPolicy{}, fmt.Errorf("incremental %q cursor column %q is not eligible for durable versioning", resource, selected.Name)
	}
	return VersionPolicy{
		Strategy: VersionCursor,
		Field:    selected.Name,
		Logical:  selected.Logical,
		Native:   selected.Native,
	}, nil
}

// ResolveWriteVersionPolicy binds the version strategy for one source-to-sink
// resource. Non-upsert writes do not need a version; snapshot upserts and
// incremental sources without cursor metadata use insertion order.
func ResolveWriteVersionPolicy(
	ctx context.Context,
	src Source,
	resource string,
	config ResourceCursorConfig,
	sourceMode ReadMode,
	writeMode WriteMode,
) (VersionPolicy, error) {
	if writeMode != WriteUpsert {
		return VersionPolicy{}, nil
	}
	fallback := VersionPolicy{Strategy: VersionInsertOrder}
	if sourceMode != ModeIncremental {
		return fallback, nil
	}
	version, err := ResolveCursorVersionPolicy(ctx, src, resource, config)
	if err != nil {
		return VersionPolicy{}, err
	}
	if version.Strategy == "" {
		return fallback, nil
	}
	return version, nil
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

// ReplicationMode is how a connection replicates, decided at source creation:
// query-based reads or the change stream.
type ReplicationMode string

// The replication modes.
const (
	ReplicationStandard ReplicationMode = "standard"
	ReplicationCDC      ReplicationMode = "cdc"
)

// ReplicationAware lets a source report which replication mode a connection
// config selects. Sources without the contract are always standard.
type ReplicationAware interface {
	Replication(cfg Config) ReplicationMode
}

// ReplicationOf resolves a connection's replication mode from its source.
func ReplicationOf(src Source, cfg Config) ReplicationMode {
	if aware, ok := src.(ReplicationAware); ok {
		return aware.Replication(cfg)
	}
	return ReplicationStandard
}

// LiveValidatable is the optional contract for probing connectivity with a
// config before any run uses it. Both sources and sinks may implement it.
type LiveValidatable interface {
	TestConnection(ctx context.Context, cfg Config) error
}

// ConfigValidatable is the optional contract for connector-specific, pure
// configuration validation. Unlike LiveValidatable it must not access the
// network, so the API can safely use it while accepting connection settings.
type ConfigValidatable interface {
	Validate(cfg Config) error
}

// ConnectorSpec is a source's self-description: identity, supported modes and
// policies, config schema, and resource capabilities. It powers the catalog.
type ConnectorSpec struct {
	Name           string
	DisplayName    string
	Description    string
	DarkLogoURL    string
	LightLogoURL   string
	Version        string
	Maturity       ConnectorMaturity
	Modes          []ReadMode
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
	Enum     []EnumOption
	Help     string
	Scope    FieldScope
	Secret   bool
	// VisibleWhen conditionally includes this field based on a sibling field.
	VisibleWhen *FieldCondition
	// Fields describes the members of an object field. It is empty for scalar
	// fields and for intentionally free-form objects.
	Fields []ConfigField
}

// EnumOption is one ordered choice for an enum or list config field.
type EnumOption struct {
	Value string
	Label string
}

// FieldCondition controls whether a config field applies based on a sibling's
// string value.
type FieldCondition struct {
	Field  string
	Values []string
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
	// FieldList is an ordered list of strings. Enum contains the selectable
	// values when the field is rendered as a checkbox group.
	FieldList
)

// FieldScope is the scope for a config field, aligning with either a connection config or a pipeline config.
// Connection == Only used to establish a connection to connector, reusable for all runs
// Pipeline == Varies based on pipeline
// For example: postgres has a dsn to connect (connection scope) and a table the data should land in (pipeline scope)
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

// ReadMode is how a source reads: full scan, incremental from a
// cursor, or CDC.
type ReadMode int

// The replication modes.
const (
	ModeFull ReadMode = iota
	ModeIncremental
	ModeCDC
)

// ResourceCapabilities advertises what a source can do per resource.
type ResourceCapabilities struct{ Discoverable, PerResourceCursor bool }

// ExtractOpts scopes one extraction: which resources, how fast. Read behavior
// per resource is carried by the checkpoint plan handed to ExtractFrom, not by
// a run-wide mode.
type ExtractOpts struct {
	Resources   []string
	Selectors   []string
	Limit       int // 0 = unbounded
	Parallelism int
	Observe     SourceObserver
}

// ChangeExtractOpts scopes one CDC extraction: resources plus the checkpoints
// to resume each from.
type ChangeExtractOpts struct {
	Resources   []string
	Checkpoints map[string]Checkpoint
	Limit       int
	Observe     SourceObserver
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

// SourceProgressKind identifies a non-terminal extraction signal. Run and
// resource lifecycle state remains runner-owned; these signals describe work
// only the source can observe directly.
type SourceProgressKind uint8

// Source progress kinds identify the source-local signals an observer can report.
const (
	SourceProgressPageFetched SourceProgressKind = iota + 1
	SourceProgressFanOutStarted
	SourceProgressWatermarkAdvanced
	SourceProgressRateLimited
	SourceProgressRetryExhausted
)

// SourceProgress carries source-local extraction progress to the runner. Fields
// are populated according to Kind. Checkpoint on WatermarkAdvanced describes an
// observed cursor only; durable checkpoint persistence remains tracker-owned.
type SourceProgress struct {
	Kind         SourceProgressKind
	Resource     string
	Records      int64
	Bytes        int64
	URI          string
	ParentsTotal int64
	RetryAfter   time.Duration
	Checkpoint   *CheckpointData
	Error        string
}

// SourceObserver receives progress concurrently when a source extracts more
// than one resource or fans out. Implementations must be concurrency-safe.
type SourceObserver func(SourceProgress)

// Report delivers progress when an observer is configured. A nil observer is
// a safe no-op so sources can report without branching at every call site.
func (o SourceObserver) Report(progress SourceProgress) {
	if o != nil {
		o(progress)
	}
}
