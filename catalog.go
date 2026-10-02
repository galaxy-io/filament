package filament

import (
	"context"
	"errors"
)

// ErrUnsupported reports that a connector does not implement the optional
// contract a Catalog call needs.
var ErrUnsupported = errors.New("unsupported by connector")

// ErrConfigure reports that a connector refused the config it was given.
var ErrConfigure = errors.New("connector rejected configuration")

// Catalog is everything the control binaries need from connectors: the specs
// that drive the UI and validation, the driver facts the compiler reads, and
// the live inspection calls that reach the connected system. A
// local Catalog wraps the registries; a remote one forwards to the worker
// serving it so the API never links a driver.
type Catalog interface {
	SourceSpecs(ctx context.Context) ([]ConnectorSpec, error)
	SinkSpecs(ctx context.Context) ([]SinkSpec, error)
	SourceSpec(ctx context.Context, name string) (ConnectorSpec, error)
	SinkSpec(ctx context.Context, name string) (SinkSpec, error)

	// PlanReplicationStream asks a streaming source to plan its consumer.
	// Sources that do not plan streams return ErrUnsupported.
	PlanReplicationStream(ctx context.Context, source string, req ReplicationStreamPlanningRequest) (ReplicationStreamPlan, error)

	// SourceContracts and SinkContracts report which optional execution
	// contracts a driver implements.
	SourceContracts(ctx context.Context, name string) (SourceContracts, error)
	SinkContracts(ctx context.Context, name string) (SinkContracts, error)

	// Validate runs the connector's pure config validation.
	Validate(ctx context.Context, kind ConnectorKind, name string, cfg Config) error
	// TestConnection runs the connector's live probe. Connectors without one
	// pass.
	TestConnection(ctx context.Context, kind ConnectorKind, name string, cfg Config) error
	// Discover configures the source and lists its resources.
	Discover(ctx context.Context, source string, cfg Config, opts DiscoverOpts) (DiscoverResult, error)
	// Inspect configures the source once and reports each resource's primary
	// key and columns. An empty resources list inspects every selectable
	// discovered resource, or returns ErrUnsupported when the source cannot
	// discover.
	Inspect(ctx context.Context, source string, cfg Config, resources []string) ([]ResourceInspection, error)
}

// SourceContracts reports which optional execution contracts a source's
// driver implements. A spec cannot carry them: they are facts about the type.
type SourceContracts struct {
	// PlansStreams is ReplicationStreamPlanner.
	PlansStreams bool
	// Streams is StreamSource.
	Streams bool
}

// SinkContracts is SourceContracts for sinks.
type SinkContracts struct {
	// Streams is StreamingSink.
	Streams bool
}

// SourceContractsOf reads the contracts off a live source.
func SourceContractsOf(source Source) SourceContracts {
	_, plans := source.(ReplicationStreamPlanner)
	_, streams := source.(StreamSource)
	return SourceContracts{PlansStreams: plans, Streams: streams}
}

// SinkContractsOf reads the contracts off a live sink.
func SinkContractsOf(sink Sink) SinkContracts {
	_, streams := sink.(StreamingSink)
	return SinkContracts{Streams: streams}
}

// ConnectorPair is a route's source and sink as a Catalog describes them.
type ConnectorPair struct {
	Source          ConnectorSpec
	SourceContracts SourceContracts
	Sink            SinkSpec
	SinkContracts   SinkContracts
}

// PairOf loads both ends of a route from the catalog.
func PairOf(ctx context.Context, catalog Catalog, source, sink string) (ConnectorPair, error) {
	var pair ConnectorPair
	var err error
	if pair.Source, err = catalog.SourceSpec(ctx, source); err != nil {
		return pair, err
	}
	if pair.SourceContracts, err = catalog.SourceContracts(ctx, source); err != nil {
		return pair, err
	}
	if pair.Sink, err = catalog.SinkSpec(ctx, sink); err != nil {
		return pair, err
	}
	pair.SinkContracts, err = catalog.SinkContracts(ctx, sink)
	return pair, err
}

// ResourceInspection is one resource's planning facts.
type ResourceInspection struct {
	Name string
	// PrimaryKey is the resource's key, or nil when unknown. PrimaryKeyErr
	// holds a failed schema read.
	PrimaryKey    []string
	PrimaryKeyErr error
	// Columns are the resource's fields. Ranked marks them as annotated with
	// cursor eligibility. ColumnsErr is ErrUnsupported when the source exposes
	// neither cursor columns nor a schema, otherwise the read error.
	Columns    []CursorColumn
	Ranked     bool
	ColumnsErr error
	// ManagedIncremental marks a resource whose incremental state the source
	// owns, so it needs no cursor column.
	ManagedIncremental bool
}
