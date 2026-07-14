package ingestion

import (
	"context"
	"errors"
	"time"

	"github.com/galaxy-io/filament/eventbus"
)

// DataStore persists run, resource, checkpoint, and dedup state — the
// control plane's system of record.
type DataStore interface {
	SaveRun(ctx context.Context, s RunState) error
	LoadRun(ctx context.Context, id RunID) (RunState, error)
	ListRuns(ctx context.Context, f RunFilter) ([]RunState, error)

	UpsertResource(ctx context.Context, rs ResourceState) error // enabled toggle + progress
	ListResources(ctx context.Context, id RunID) ([]ResourceState, error)

	SaveCheckpoint(ctx context.Context, id RunID, cp Checkpoint) error
	LoadCheckpoint(ctx context.Context, id RunID, resource string) (Checkpoint, error)

	DedupSeen(ctx context.Context, tenant string, run RunID, seq uint64) (bool, error)
	Name() string
}

// ScheduleStore persists schedules and hands out due ones under a claim, so
// concurrent schedulers never double-fire.
type ScheduleStore interface {
	SaveSchedule(ctx context.Context, s ScheduleState) error
	LoadSchedule(ctx context.Context, id ScheduleID) (ScheduleState, error)
	ListSchedules(ctx context.Context, f ScheduleFilter) ([]ScheduleState, error)
	DeleteSchedule(ctx context.Context, id ScheduleID) error
	ClaimDue(ctx context.Context, now time.Time, limit int) ([]ScheduleState, error) // SELECT … FOR UPDATE SKIP LOCKED
	MarkFired(ctx context.Context, id ScheduleID, at time.Time) error
}

// Scheduler triggers one-off runs and manages the lifecycle of recurring
// schedules.
type Scheduler interface {
	Trigger(ctx context.Context, req RunRequest) (RunHandle, error) // one-off, immediate
	Register(ctx context.Context, spec ScheduleSpec) (ScheduleID, error)
	Update(ctx context.Context, id ScheduleID, spec ScheduleSpec) error
	Pause(ctx context.Context, id ScheduleID) error
	Resume(ctx context.Context, id ScheduleID) error
	Delete(ctx context.Context, id ScheduleID) error
	Get(ctx context.Context, id ScheduleID) (ScheduleState, error)
	List(ctx context.Context, f ScheduleFilter) ([]ScheduleState, error)
	Name() string
}

// ScheduleSpec defines a recurring run: the cron timing plus the request to
// fire and the overlap/catchup behavior when ticks collide or are missed.
type ScheduleSpec struct {
	Tenant   TenantID
	Name     string
	Cron     string
	Timezone string
	Jitter   time.Duration
	Overlap  OverlapPolicy
	Catchup  CatchupPolicy
	Request  RunRequest
	Enabled  bool
}

// OverlapPolicy decides what a tick does when the previous run is still
// in flight.
type OverlapPolicy int

// The overlap policies; OverlapSkip drops the tick.
const (
	OverlapSkip           OverlapPolicy = iota
	OverlapAllow                        // run concurrently
	OverlapBufferOne                    // queue exactly one
	OverlapCancelPrevious               // cancel the in-flight run, start fresh
)

// CatchupPolicy decides what happens to ticks missed while the scheduler was
// down.
type CatchupPolicy int

// The catchup policies.
const (
	CatchupSkip    CatchupPolicy = iota // ignore missed ticks
	CatchupRunOnce                      // one make-up run after downtime
	CatchupRunAll                       // fire every missed tick
)

// ScheduleState is a persisted schedule plus derived timing.
type ScheduleState struct {
	ID         ScheduleID
	Spec       ScheduleSpec
	Enabled    bool
	LastFired  *time.Time
	NextFire   *time.Time
	LastRun    RunID
	LastStatus RunStatus
	CreatedAt  time.Time
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
