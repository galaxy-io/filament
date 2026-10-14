package filament

import (
	"context"
	"errors"
)

// ErrUnsupported reports an optional contract the connector does not offer,
// such as discovery or a connectivity probe.
var ErrUnsupported = errors.New("unsupported by connector")

// ErrConfigure reports a connector rejecting its configuration, either when
// validating it or when connecting with it. Workers return it as a
// ConfigureError so the connector's own reason survives intact.
var ErrConfigure = errors.New("connector rejected configuration")

// ConfigureError is ErrConfigure carrying the connector's reason verbatim.
type ConfigureError struct{ Reason error }

// Error is the connector's reason, unprefixed.
func (e *ConfigureError) Error() string { return e.Reason.Error() }

// Unwrap exposes the connector's reason to errors.Is and errors.As.
func (e *ConfigureError) Unwrap() error { return e.Reason }

// Is matches ErrConfigure so callers can branch without knowing the type.
func (e *ConfigureError) Is(target error) bool { return target == ErrConfigure }

// ConnectorRef names one registered connector by kind and name.
type ConnectorRef struct {
	Kind ConnectorKind
	Name string
}

// Catalog is every registered connector spec. Sources include alias entries
// with AliasTarget set so readers can tell them from concrete registrations.
type Catalog struct {
	Sources []ConnectorSpec
	Sinks   []SinkSpec
}

// InspectStatus is the outcome of inspecting one resource.
type InspectStatus int

// The inspect statuses.
const (
	// InspectOK means PrimaryKey, Columns and ManagedIncremental are populated.
	InspectOK InspectStatus = iota
	// InspectUnsupported means the source cannot list columns; PrimaryKey and
	// ManagedIncremental are still populated where the source reports them.
	InspectUnsupported
	// InspectFailed means the source errored for this resource; Message says why.
	InspectFailed
)

// Inspection is what a configured source reports about one resource.
type Inspection struct {
	Name    string
	Status  InspectStatus
	Message string

	PrimaryKey []string
	// Columns are the resource's schema fields with cursor eligibility laid
	// over them when Ranked is true; cursor columns the schema lacks follow.
	// Without a schema they are the cursor columns alone.
	Columns            []CursorColumn
	Ranked             bool
	ManagedIncremental bool
	// Schema is nil when the source provides none.
	Schema *RecordSchema
}

// Worker answers every connector question the control services ask: the
// registered specs, and live operations that configure a connector with a
// caller-supplied config. Unknown connectors are ErrNotFound, missing optional
// contracts are ErrUnsupported, and a connector rejecting its config is
// ErrConfigure; every other error is a host failure.
type Worker interface {
	Describe(ctx context.Context) (Catalog, error)
	SourceSpec(ctx context.Context, name string) (ConnectorSpec, error)
	SinkSpec(ctx context.Context, name string) (SinkSpec, error)

	Validate(ctx context.Context, ref ConnectorRef, cfg Config) error
	TestConnection(ctx context.Context, ref ConnectorRef, cfg Config) error
	Discover(ctx context.Context, source string, cfg Config, opts DiscoverOpts) ([]Resource, error)
	// Inspect reports on the named resources, or on every selectable resource
	// when none are named.
	Inspect(ctx context.Context, source string, cfg Config, resources []string) ([]Inspection, error)
	PlanReplicationStream(ctx context.Context, source string, req ReplicationStreamPlanningRequest) (ReplicationStreamPlan, error)
}
