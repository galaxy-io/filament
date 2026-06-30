package pipeline

import (
	"context"
	"io"
	"log/slog"

	"github.com/galaxy-io/filament/connectors/http/internal/pipeline/integrity"
)

// ConnectorTier classifies the integration depth of a connector.
type ConnectorTier int

const (
	// TierNative uses a first-party SDK (e.g. Google Drive, Salesforce Bulk API).
	TierNative ConnectorTier = iota + 1
	// TierHTTP is driven by a declarative YAML manifest over generic HTTP.
	TierHTTP
)

func (t ConnectorTier) String() string {
	switch t {
	case TierNative:
		return "native"
	case TierHTTP:
		return "http"
	default:
		return "unknown"
	}
}

// ConnectorMode describes an extraction paradigm a connector supports.
type ConnectorMode int

const (
	ModeBatch  ConnectorMode = iota + 1 // full snapshot / paginated extraction
	ModeStream                          // long-lived NDJSON stream
	ModeCDC                             // change data capture (WAL, webhooks, etc.)
)

func (m ConnectorMode) String() string {
	switch m {
	case ModeBatch:
		return "batch"
	case ModeStream:
		return "stream"
	case ModeCDC:
		return "cdc"
	default:
		return "unknown"
	}
}

// ConnectorSpec describes a connector's capabilities and configuration surface.
type ConnectorSpec struct {
	Name  string
	Tier  ConnectorTier
	Modes []ConnectorMode
}

// OutputWriter receives pre-marshaled NDJSON lines for a given resource.
type OutputWriter interface {
	WriteLines(ctx context.Context, resource string, lines [][]byte) error
	Close(ctx context.Context) error
}

// ContentStore writes raw file content as sidecar objects and returns a URI.
type ContentStore interface {
	Put(ctx context.Context, key string, data io.Reader, size int64) (uri string, err error)
}

// ExtractOptions is passed to Connector.Extract.
type ExtractOptions struct {
	Sink     *RecordSink
	Content  ContentStore
	Logger   *slog.Logger
	Reporter Reporter
	// Checkpoint, when non-nil, is the write target for connector-side state
	// that needs to survive across runs (e.g. incremental watermarks).
	// Connectors that don't track resumable state may ignore it. The runner
	// is responsible for persisting the checkpoint after Extract returns.
	Checkpoint *integrity.PipelineCheckpoint
	// EnabledResources, when non-nil, restricts extraction to the given set of
	// discovered resources (e.g. specific slack channels, gdrive folders).
	// nil means "extract everything" — legacy behavior. Connectors that do not
	// honor this field will have unwanted records filtered post-extraction by
	// the runner, but pushdown is strongly preferred.
	EnabledResources []ResourceRef
	// Resources restricts extraction to manifest resource names. This is used
	// for non-discovery connectors where resources are static manifest entries.
	Resources []string
	// ResumeCursors seeds pagination for resources when the embedding runtime
	// owns checkpoint persistence. Unlike ExtractFrom's PipelineCheckpoint path,
	// this does not skip completed resources; it only starts the first request
	// from the supplied cursor.
	ResumeCursors map[string]string
	// ResumeWatermarks seeds incremental resources when the embedding runtime
	// owns checkpoint persistence. Outer key is resource name, inner key is the
	// manifest checkpoint key.
	ResumeWatermarks map[string]map[string]string
}

// ResourceRef identifies a single discovered resource by connector-side kind+id.
type ResourceRef struct {
	Kind string
	ID   string
}

// Resource is a discoverable extraction target — a slack channel, gdrive folder,
// github repo, etc. Connectors expose these via the optional Discoverer interface
// so users can enable/disable them per-connection in the UI.
type Resource struct {
	Kind           string            // connector-defined, e.g. "channel", "folder"
	ID             string            // stable connector-side identifier
	Name           string            // human-readable display name
	ParentID       string            // empty == root
	Group          string            // optional bucket label for the picker (e.g. "schema: public")
	DefaultEnabled bool              // connector's policy for newly-discovered resources
	Metadata       map[string]string // surfaced in UI (is_private, member_count, ...)
}

// DiscoverOptions controls a discovery pass.
type DiscoverOptions struct {
	Logger *slog.Logger
	// SinceToken, when non-empty, resumes discovery from the previous
	// DiscoverResult.NextToken. Connectors that cannot resume should ignore it.
	SinceToken string
	// Kinds optionally restricts discovery to the listed resource kinds.
	// Empty means all kinds the connector supports.
	Kinds []string
}

// DiscoverResult is the outcome of one discovery pass. NextToken being non-empty
// signals the caller to invoke Discover again with SinceToken set to continue.
type DiscoverResult struct {
	Resources []Resource
	NextToken string
}

// Connector is the core extraction interface — validate, configure, extract, teardown.
type Connector interface {
	Spec() ConnectorSpec
	Validate() error
	Configure(ctx context.Context) error
	Extract(ctx context.Context, opts ExtractOptions) error
	Teardown(ctx context.Context) error
}

// ResumableConnector is an optional interface that connectors can implement
// to support resuming extraction from a previous checkpoint. The runner
// checks for this at runtime and falls back to full Extract if not supported.
type ResumableConnector interface {
	Connector
	ExtractFrom(ctx context.Context, opts ExtractOptions, prev *integrity.PipelineCheckpoint) error
}

// Discoverer is an OPTIONAL interface a connector may implement to enumerate
// the user-toggleable resources reachable from its credentials (slack channels,
// gdrive folders, github repos, ...). Connectors without this interface are
// treated as a single implicit resource and cannot be filtered per-resource.
type Discoverer interface {
	Connector
	Discover(ctx context.Context, opts DiscoverOptions) (*DiscoverResult, error)
}

// Reporter receives extraction progress events.
type Reporter interface {
	Report(Event)
}

// NoopReporter discards all events.
type NoopReporter struct{}

func (NoopReporter) Report(Event) {}
