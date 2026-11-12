package filament

import (
	"context"
	"errors"
	"fmt"
)

// ErrResourceQuarantined reports that a resource was stopped by an earlier
// failure while the rest of the run continues. A source that receives it from
// a RowWriter stops that resource only and keeps extracting the others.
var ErrResourceQuarantined = errors.New("resource quarantined")

// ResourceError scopes a failure to one resource. A sink returns it from Apply
// when the failure leaves the session healthy for every other resource. The
// pipeline returns it to a source whose resource was quarantined.
type ResourceError struct {
	Resource string
	Err      error
}

func (e *ResourceError) Error() string {
	return fmt.Sprintf("resource %q: %v", e.Resource, e.Err)
}

func (e *ResourceError) Unwrap() error { return e.Err }

// ResourceFailureReporter is implemented by the RecordSink a source receives
// when the run isolates resource failures. A source reports a failure that
// leaves its other resources readable, stops that resource, keeps extracting
// the rest, and returns nil once every other resource is complete. A non-nil
// error returned from extraction always fails the whole run, because the
// engine cannot tell which other resources it left unfinished.
type ResourceFailureReporter interface {
	FailResource(resource string, err error)
}

// ResourceIsolatingSource is implemented by a source that, when a RowWriter
// returns ErrResourceQuarantined, stops that resource and keeps extracting the
// others. Under isolation the engine returns that error only to such a source.
// Any other source keeps writing the quarantined resource and the engine
// discards its rows, so an unchanged source never stops the run because of
// a failure the engine already contained.
type ResourceIsolatingSource interface {
	IsolatesResourceFailures() bool
}

// ResourceIsolatingSink is a sink whose session can contain one resource's
// failure. IsolatesResources is asked after Open, because whether a session
// can isolate may depend on the run (for example a single shared transaction
// cannot). When it reports true:
//   - Apply returns a *ResourceError for a failure confined to one resource,
//     and the session stays usable for Apply and Commit on every other one.
//     Any other error still ends the session.
//   - AbortResource discards that resource's uncommitted writes before the
//     session commits the rest. Writes already durable on Apply are not undone.
type ResourceIsolatingSink interface {
	IsolatesResources() bool
	AbortResource(ctx context.Context, resource string) error
}
