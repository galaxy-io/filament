package filament

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/eventbus"
)

// DataStore persists run, resource, checkpoint, and dedup state — the
// control plane's system of record.
type DataStore interface {
	Ping(ctx context.Context) error // reachability, for readiness probes
	EnsureTenant(ctx context.Context, id TenantID, name string) error

	SaveRun(ctx context.Context, s RunState) error
	LoadRun(ctx context.Context, id RunID) (RunState, error)
	ListRuns(ctx context.Context, f RunFilter) ([]RunState, error)
	// DeleteRun removes a run and its resources. Only the scheduler calls it, to
	// reap pre-created RunScheduled rows; deleting a run that ever executed would
	// discard history. Deleting a missing run is a no-op.
	DeleteRun(ctx context.Context, id RunID) error

	UpsertResource(ctx context.Context, rs ResourceState) error // enabled toggle + progress
	ListResources(ctx context.Context, id RunID) ([]ResourceState, error)

	SaveCheckpoint(ctx context.Context, id RunID, cp Checkpoint) error
	LoadCheckpoint(ctx context.Context, id RunID, resource string) (Checkpoint, error)
	SaveResourceCheckpoint(ctx context.Context, state ResourceCheckpointState) error
	LoadResourceCheckpoint(ctx context.Context, key ResourceCheckpointKey) (ResourceCheckpointState, error)
	DeleteResourceCheckpoint(ctx context.Context, key ResourceCheckpointKey) error

	DedupSeen(ctx context.Context, tenant string, run RunID, seq uint64) (bool, error)

	CreateConnection(ctx context.Context, c Connection) (Connection, error)
	UpdateConnection(ctx context.Context, c Connection) (Connection, error)
	LoadConnection(ctx context.Context, id string) (Connection, error)
	ListConnections(ctx context.Context, f ConnectionFilter) ([]Connection, error)
	DeleteConnection(ctx context.Context, id string) error

	CreatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error)
	CreatePipelineVersion(ctx context.Context, pipelineID string, v *ingestionv1.PipelineVersion) (*ingestionv1.PipelineVersion, error)
	UpdatePipeline(ctx context.Context, p *ingestionv1.Pipeline) (*ingestionv1.Pipeline, error)
	// LoadPipeline includes soft-deleted pipelines; check DeletedAt before
	// mutating or running one.
	LoadPipeline(ctx context.Context, id string) (*ingestionv1.Pipeline, error)
	LoadPipelineVersion(ctx context.Context, pipelineID string, version int64) (*ingestionv1.PipelineVersion, error)
	ListPipelineVersions(ctx context.Context, pipelineID string) ([]*ingestionv1.PipelineVersion, error)
	ListPipelines(ctx context.Context, f PipelineFilter) ([]*ingestionv1.Pipeline, error)
	DeletePipeline(ctx context.Context, id string) error
	Name() string
}

// ResourceCheckpointKey identifies durable progress shared by runs of one
// immutable pipeline route. Resource is deliberately part of the key so each
// table advances independently.
type ResourceCheckpointKey struct {
	PipelineID        string
	PipelineVersionID int64
	Route             string
	Resource          string
}

// ResourceCheckpointState is the durable cursor plus its most recent writer.
// Connector-specific phases and cursor shapes remain opaque inside Checkpoint.
type ResourceCheckpointState struct {
	Key        ResourceCheckpointKey
	Run        RunID
	Checkpoint Checkpoint
	UpdatedAt  time.Time
}

// ConnectorKind identifies which registry owns a reusable connection.
type ConnectorKind int

// Connector kinds identify which registry owns a reusable connection.
const (
	ConnectorKindUnspecified ConnectorKind = iota
	ConnectorKindSource
	ConnectorKindSink
)

// Connection is the persistence-domain representation of reusable connector config.
// Config contains no plaintext fields declared as FieldSecret.
type Connection struct {
	ID         string
	Tenant     string
	Kind       ConnectorKind
	Name       string
	Connector  string
	Config     map[string]any
	SecretRefs map[string]string
	Version    int64
	// DeletedAt is unix milliseconds, zero when the connection is live.
	DeletedAt int64
}

// ConnectionFilter narrows a connection listing by tenant and/or kind.
type ConnectionFilter struct {
	Tenant         string
	Kind           ConnectorKind
	IncludeDeleted bool
}

// PipelineFilter narrows a pipeline listing by tenant.
type PipelineFilter struct {
	Tenant         string
	IncludeDeleted bool
}

// ErrVersionConflict indicates an optimistic-lock mismatch.
var ErrVersionConflict = errors.New("version conflict")

// ScheduleStore persists schedules and hands out due ones under a claim, so
// concurrent schedulers never double-fire.
type ScheduleStore interface {
	SaveSchedule(ctx context.Context, s ScheduleState) error
	LoadSchedule(ctx context.Context, id ScheduleID) (ScheduleState, error)
	LoadPipelineSchedule(ctx context.Context, pipelineID string) (ScheduleState, error)
	ListSchedules(ctx context.Context, f ScheduleFilter) ([]ScheduleState, error)
	DeleteSchedule(ctx context.Context, id ScheduleID) error
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]ScheduleState, error) // SELECT … FOR UPDATE SKIP LOCKED
	ReleaseScheduleClaim(ctx context.Context, id ScheduleID) error
}

// PipelineScheduleStore atomically creates a pipeline and its optional primary
// schedule.
type PipelineScheduleStore interface {
	ScheduleStore
	CreatePipelineWithSchedule(ctx context.Context, p *ingestionv1.Pipeline, schedule *ScheduleState) (*ingestionv1.Pipeline, error)
}

// MetricsStore is an optional DataStore capability (see PipelineScheduleStore)
// for run metrics queries, backing metrics.v1.MetricsService. Ping mirrors
// DataStore's (readiness probes); a backend that needs cleanup on shutdown
// additionally implements io.Closer (checked optionally, same as DataStore
// backends — see cmd/server/main.go).
type MetricsStore interface {
	Ping(ctx context.Context) error
	QueryRunTimeseries(ctx context.Context, q RunTimeseriesQuery) ([]RunTimeseries, error)
	QueryRunAggregate(ctx context.Context, q RunAggregateQuery) ([]RunAggregateRow, error)
}

// Scheduler manages the lifecycle of recurring pipeline schedules.
type Scheduler interface {
	Register(ctx context.Context, spec ScheduleSpec) (ScheduleID, error)
	Update(ctx context.Context, id ScheduleID, spec ScheduleSpec) error
	Pause(ctx context.Context, id ScheduleID) error
	Resume(ctx context.Context, id ScheduleID) error
	Delete(ctx context.Context, id ScheduleID) error
	Get(ctx context.Context, id ScheduleID) (ScheduleState, error)
	List(ctx context.Context, f ScheduleFilter) ([]ScheduleState, error)
	Name() string
}

// ScheduleSpec defines when a pipeline runs and whether occurrences may overlap.
type ScheduleSpec struct {
	Tenant     TenantID
	Name       string
	PipelineID string
	Cron       string
	Timezone   string
	Overlap    OverlapPolicy
	Enabled    bool
}

// OverlapPolicy decides what a tick does when the previous run is still
// in flight.
type OverlapPolicy int

// The supported overlap policies.
const (
	OverlapSkip  OverlapPolicy = iota
	OverlapAllow               // run concurrently
)

// ScheduleState is a persisted schedule plus derived timing.
type ScheduleState struct {
	ID        ScheduleID
	Spec      ScheduleSpec
	Enabled   bool
	LastFired *time.Time
	NextFire  *time.Time
	CreatedAt time.Time
}

// ScheduleFilter narrows a schedule listing; zero fields match everything.
type ScheduleFilter struct {
	Tenant  TenantID
	Enabled *bool
	Limit   int
	Cursor  string
}

// Secrets resolves opaque references to secret material, keeping plaintext
// out of configs and stores.
type Secrets interface {
	Read(ctx context.Context, ref string) (Secret, error)
	Write(ctx context.Context, ref string, s Secret) error
	Delete(ctx context.Context, ref string) error
	Name() string
}

// Secret is plaintext, in-memory only; never logged, never on the bus.
type Secret struct {
	Value []byte
	Meta  map[string]string
}

// ConnectionSecretPrefix namespaces the secret refs the connection API mints on
// a tenant's behalf: connections/<tenant>/<connID>/<field>/v<version>.
const ConnectionSecretPrefix = "filament/"

// ConnectionSecretRef builds the canonical ref for a connection-managed secret.
func ConnectionSecretRef(tenant, connID, field string, version int64) string {
	return fmt.Sprintf("%s%s/%s/%s/v%d", ConnectionSecretPrefix, tenant, connID, field, version)
}

// ValidateConnectionSecretRef enforces the sole cross-tenant isolation rule for
// the shared, ref-keyed secret store: a ref in the connection-managed namespace
// (connections/<tenant>/...) may only be read or written by its owning tenant.
// Refs outside that namespace (e.g. env-style) carry no tenant and pass through.
func ValidateConnectionSecretRef(ref string, tenant TenantID) error {
	if !strings.HasPrefix(ref, ConnectionSecretPrefix) {
		return nil
	}
	owner, _, ok := strings.Cut(ref[len(ConnectionSecretPrefix):], "/")
	if !ok || owner == "" {
		return fmt.Errorf("malformed connection secret ref %q", ref)
	}
	if owner != string(tenant) {
		return fmt.Errorf("secret ref %q is not owned by tenant %q", ref, tenant)
	}
	return nil
}

// Dispatcher routes a resolved RunSpec to whatever executes it.
type Dispatcher interface {
	Dispatch(ctx context.Context, spec RunSpec) (RunHandle, error) // bus | inline | binary | k8s
	Name() string
}

// Runtime executes runs and delivers lifecycle signals to them.
type Runtime interface {
	Run(ctx context.Context, spec RunSpec) (RunHandle, error)
	Signal(ctx context.Context, h RunHandle, sig Signal) error
	Name() string
}

// RunHandle is a caller's grip on a dispatched run: poll its status or block
// for the result.
type RunHandle interface {
	ID() RunID
	Status(ctx context.Context) (RunStatus, error)
	Wait(ctx context.Context) (RunResult, error)
}

// Signal is a lifecycle command sent to a running run.
type Signal int

// The run signals.
const (
	SignalPause Signal = iota
	SignalResume
	SignalCancel
)

type (
	// SourceFactory constructs a fresh Source instance per resolve.
	SourceFactory func() Source
	// SinkFactory constructs a fresh Sink instance per resolve.
	SinkFactory func() Sink
)

// SourceRegistry maps source names to factories and exposes their specs for
// the catalog.
type SourceRegistry interface {
	Register(name string, f SourceFactory)
	Resolve(name string) (Source, error)
	Specs() []ConnectorSpec
}

// SinkRegistry maps sink names to factories and exposes their specs for the
// catalog.
type SinkRegistry interface {
	Register(name string, f SinkFactory)
	Resolve(name string) (Sink, error)
	Specs() []SinkSpec
}

// mapConfig is the concrete Config backing a provider's Ref.Config (a decoded
// map[string]any from YAML/JSON). It reads tolerantly across the numeric forms
// JSON round-trips produce. Secret resolution is not wired here yet — Secret and
// SecretRef return the raw value, so a secrets provider can be layered in later.
type mapConfig map[string]any

// NewConfig adapts a decoded map into a Config. A nil map yields an empty Config
// where every lookup misses (Has → false, scalars → zero).
func NewConfig(m map[string]any) Config { return mapConfig(m) }

var _ Config = mapConfig(nil)

func (c mapConfig) Has(key string) bool {
	_, ok := c[key]
	return ok
}

func (c mapConfig) String(key string) string {
	if s, ok := c[key].(string); ok {
		return s
	}
	return ""
}

// Int tolerates the int / int64 / float64 forms a decoder may produce.
func (c mapConfig) Int(key string) int {
	switch n := c[key].(type) {
	case int:
		return n
	case int64:
		return int(n)
	case float64:
		return int(n)
	default:
		return 0
	}
}

func (c mapConfig) Bool(key string) bool {
	b, _ := c[key].(bool)
	return b
}

// Duration parses a string value with time.ParseDuration, or treats a numeric
// value as a count of seconds.
func (c mapConfig) Duration(key string) time.Duration {
	switch v := c[key].(type) {
	case string:
		d, err := time.ParseDuration(v)
		if err != nil {
			return 0
		}
		return d
	case int:
		return time.Duration(v) * time.Second
	case int64:
		return time.Duration(v) * time.Second
	case float64:
		return time.Duration(v) * time.Second
	default:
		return 0
	}
}

// Secret returns the raw configured value. Resolution against a Secrets provider
// is layered on by the engine once that wiring lands; until then a config may
// carry an inline value for dev.
func (c mapConfig) Secret(key string) string { return c.String(key) }

// SecretRef returns the reference string a Secrets provider would resolve.
func (c mapConfig) SecretRef(key string) string { return c.String(key) }

// Raw returns the underlying decoded map for callers that need to iterate keys
// (e.g. passing arbitrary backend properties through to a provider library).
func (c mapConfig) Raw() map[string]any { return c }

// Sub returns a nested Config, or an empty one if the key is absent or not a map.
func (c mapConfig) Sub(key string) Config {
	if m, ok := c[key].(map[string]any); ok {
		return mapConfig(m)
	}
	return mapConfig(nil)
}

// RecordSink is where a Source pushes extracted records — the engine's inlet,
// not a data Sink.
type RecordSink interface {
	Push(r Record) error
	PushBatch(rs []Record) error
}

// Config is typed, tolerant read access to a connector's configuration; every
// accessor misses to a zero value.
type Config interface {
	String(key string) string
	Int(key string) int
	Bool(key string) bool
	Duration(key string) time.Duration
	Secret(key string) string
	SecretRef(key string) string
	Sub(key string) Config
	Has(key string) bool
	Raw() map[string]any
}

// Checkpoint is a resource's resumable cursor: keyed reads plus an immutable
// Set that returns an updated copy. CheckpointData is the concrete form.
type Checkpoint interface {
	Resource() string
	Int(key string) int
	String(key string) string
	Set(key string, v any) Checkpoint
	Raw() map[string]any
}

// Logger is structured leveled logging with field accumulation via With.
type Logger interface {
	Debug(msg string, kv ...Field)
	Info(msg string, kv ...Field)
	Warn(msg string, kv ...Field)
	Error(msg string, err error, kv ...Field)
	With(kv ...Field) Logger
}

// Metrics vends the three instrument kinds by name and labels.
type Metrics interface {
	Counter(name string, labels ...Label) Counter
	Gauge(name string, labels ...Label) Gauge
	Histogram(name string, labels ...Label) Histogram
}

// Tracer starts spans; the returned context carries the span for nesting.
type Tracer interface {
	Start(ctx context.Context, name string) (context.Context, Span)
}

// ── Observability primitives ────────────────────────────────────────────────

// Field is a structured log field.
type Field struct {
	Key   string
	Value any
}

// Label is a metric label.
type Label struct{ Key, Value string }

// Counter is a monotonically increasing metric.
type Counter interface {
	Inc()
	Add(float64)
}

// Gauge is a metric that can move in both directions.
type Gauge interface {
	Set(float64)
	Inc()
	Dec()
}

// Histogram records a distribution of observed values.
type Histogram interface{ Observe(float64) }

// Span is one traced operation; End it exactly once.
type Span interface {
	End()
	SetError(err error)
	SetAttr(key string, v any)
}

// Domain identifiers. TenantID and RunID double as subject tokens, so they
// carry Valid methods.
type (
	// TenantID identifies a tenant.
	TenantID string
	// RunID identifies a run.
	RunID string
	// ScheduleID identifies a schedule.
	ScheduleID string
	// StageID identifies a Transactional sink's staging area.
	StageID string
)

// Valid reports whether the ID is usable as a subject token (see ValidToken).
// TenantID and RunID become subject tokens, so they must validate before a run
// is accepted or an event published.
func (t TenantID) Valid() error { return ValidToken(string(t)) }

// Valid reports whether the run ID is usable as a subject token (see ValidToken).
func (r RunID) Valid() error { return ValidToken(string(r)) }

// ValidToken reports whether s is usable as a single subject token, wrapping
// [ErrInvalidToken] on the first problem. See [eventbus.ValidToken].
func ValidToken(s string) error { return eventbus.ValidToken(s) }

// IsValidToken is the boolean form of ValidToken.
func IsValidToken(s string) bool { return eventbus.IsValidToken(s) }

var (
	// ErrInvalidToken is returned when an ID (tenant/run/…) can't be used as a
	// subject token. Aliased to the shared sentinel so errors.Is matches across
	// the eventbus boundary.
	ErrInvalidToken = eventbus.ErrInvalidToken

	// ErrNotFound is the sentinel a DataStore returns (wrapped) when a run,
	// resource, or checkpoint does not exist, so callers can branch with
	// errors.Is — e.g. the tracker creating run state on first sight of a fact.
	ErrNotFound = errors.New("not found")
)

// Ref names a provider (source, sink, datastore, …) together with its config
// — the indirection a RunRequest carries instead of live instances.
type Ref struct {
	Provider  string
	ConfigRef string
	Config    map[string]any

	// SecretRefs maps a provider config field to a Secrets reference
	SecretRefs map[string]string
}
