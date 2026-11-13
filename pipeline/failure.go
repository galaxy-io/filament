package pipeline

import (
	"errors"
	"fmt"
	"sort"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
)

// Scope classifies a failure for the arbiter. The zero value is ScopeSession,
// so an unclassified failure stops the run.
type Scope uint8

// The failure scopes.
const (
	// ScopeSession failures reach beyond one resource: a lost connection, a
	// poisoned sink session, a short sink receipt, a panic.
	ScopeSession Scope = iota
	// ScopeResource failures leave every other resource able to continue.
	ScopeResource
	// ScopeIntegrity failures mean written bytes cannot be trusted.
	ScopeIntegrity
)

func (s Scope) String() string {
	switch s {
	case ScopeResource:
		return "resource"
	case ScopeIntegrity:
		return "integrity"
	default:
		return "session"
	}
}

// Failure is one failure reported to the arbiter.
type Failure struct {
	Resource string
	Scope    Scope
	Err      error
}

// FailurePolicy decides what a resource-scoped failure does. The zero value
// is strict: every failure stops the run, as before isolation existed.
type FailurePolicy struct {
	// Isolate quarantines a failed resource instead of stopping the run.
	// Serial stream pipelines ignore it.
	Isolate bool
	// MaxFailedResources stops the run once more resources than this are
	// quarantined. 0 is unbounded.
	MaxFailedResources int
	// SinkIsolation reports that the sink can discard one resource's
	// uncommitted writes and keep the session usable for the rest. Without
	// it, only a resource that never reached Apply can be quarantined.
	SinkIsolation bool
	// SourceStopsQuarantined reports that the source handles
	// ErrResourceQuarantined by stopping that resource only. Without it, a
	// quarantined resource's builders accept rows and the pipeline drops
	// them, because an unaware source would return the error and fail the run.
	SourceStopsQuarantined bool
}

// Quarantine describes a resource stopped by the arbiter.
type Quarantine struct {
	Err error
	// Applied reports that at least one Apply was attempted for the resource,
	// so the sink may hold its uncommitted writes.
	Applied bool
}

// resourceGate is one resource's quarantine switch. Builders select on done;
// writers check it under mu before handing a batch to the sink, so marking a
// resource applied and quarantining it never interleave.
type resourceGate struct {
	resource string
	done     chan struct{}
	mu       sync.Mutex
	err      error
	applied  bool
}

func newResourceGate(resource string) *resourceGate {
	return &resourceGate{resource: resource, done: make(chan struct{})}
}

// failure returns the error a builder of a quarantined resource receives.
func (g *resourceGate) failure() error {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err == nil {
		return nil
	}
	return &filament.ResourceError{Resource: g.resource, Err: fmt.Errorf("%w: %w", filament.ErrResourceQuarantined, g.err)}
}

// quarantined reports whether the gate is closed. Nil gates (controls) never are.
func (g *resourceGate) quarantined() bool {
	if g == nil {
		return false
	}
	select {
	case <-g.done:
		return true
	default:
		return false
	}
}

// admit marks the resource applied unless it is quarantined. A writer calls it
// immediately before the sink sees the batch.
func (g *resourceGate) admit() bool {
	if g == nil {
		return true
	}
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.err != nil {
		return false
	}
	g.applied = true
	return true
}

// errDropped is a quarantined batch released at the inlet instead of queued.
// It never leaves the slot: the builder sees success and keeps going.
var errDropped = errors.New("pipeline: quarantined batch dropped")

// gate returns the resource's gate, creating it on first use.
func (p *Pipeline) gate(resource string) *resourceGate {
	p.gateMu.Lock()
	defer p.gateMu.Unlock()
	g := p.gates[resource]
	if g == nil {
		g = newResourceGate(resource)
		p.gates[resource] = g
	}
	return g
}

// report is the arbiter: the only path from a failure to stopping the run. A
// resource-scoped failure under an isolating policy quarantines its resource;
// every other failure, or one the policy cannot contain, stops the run.
func (p *Pipeline) report(f Failure) {
	if f.Err == nil {
		return
	}
	if reason := p.quarantine(f); reason != nil {
		p.setErr(reason)
	}
}

// quarantine closes f's resource gate when policy allows, returning nil, or
// returns the error that must stop the run instead.
func (p *Pipeline) quarantine(f Failure) error {
	if !p.policy.Isolate || p.stream != nil || f.Scope != ScopeResource || f.Resource == "" {
		return f.Err
	}
	g := p.gate(f.Resource)
	g.mu.Lock()
	if g.err != nil { // already quarantined; the first cause stands
		g.mu.Unlock()
		return nil
	}
	if g.applied && !p.policy.SinkIsolation {
		g.mu.Unlock()
		return fmt.Errorf("%w (sink %q cannot discard a failed resource's writes)", f.Err, p.sinkName())
	}
	g.err = f.Err
	close(g.done)
	g.mu.Unlock()

	failed := p.quarantinedCount.Add(1)
	if limit := p.policy.MaxFailedResources; limit > 0 && int(failed) > limit {
		return fmt.Errorf("%d resources failed, more than the limit of %d: %w", failed, limit, f.Err)
	}
	if p.log != nil {
		p.log.Warn("resource quarantined",
			filament.Field{Key: "event.name", Value: "pipeline.resource.quarantined"},
			filament.Field{Key: "resource", Value: f.Resource},
			filament.Field{Key: "error", Value: f.Err.Error()},
		)
	}
	p.publish(events.NewFact(events.ResourceQuarantined, events.Envelope{Resource: f.Resource},
		events.ResourceQuarantinedEvent{Error: f.Err.Error()}))
	return nil
}

func (p *Pipeline) sinkName() string {
	if p.sink == nil {
		return ""
	}
	return p.sink.Name()
}

// Quarantined returns every resource the arbiter stopped, with its first
// cause. Call after Wait.
func (p *Pipeline) Quarantined() map[string]Quarantine {
	p.gateMu.Lock()
	defer p.gateMu.Unlock()
	out := make(map[string]Quarantine)
	for resource, g := range p.gates {
		g.mu.Lock()
		if g.err != nil {
			out[resource] = Quarantine{Err: g.err, Applied: g.applied}
		}
		g.mu.Unlock()
	}
	return out
}

// QuarantinedResources returns the quarantined resource names, sorted.
func QuarantinedResources(q map[string]Quarantine) []string {
	out := make([]string, 0, len(q))
	for resource := range q {
		out = append(out, resource)
	}
	sort.Strings(out)
	return out
}

// classifyApply scopes a sink Apply failure. Only a sink that declared
// resource isolation can scope its own error to one resource.
func (p *Pipeline) classifyApply(err error) Scope {
	var re *filament.ResourceError
	if p.policy.SinkIsolation && errors.As(err, &re) {
		return ScopeResource
	}
	return ScopeSession
}

// isolatingInlet is the inlet a source sees under an isolating policy. It adds
// FailResource so a source can report a failure on one resource and go on.
type isolatingInlet struct{ *inlet }

var _ filament.ResourceFailureReporter = isolatingInlet{}

// FailResource quarantines resource on the source's report.
func (in isolatingInlet) FailResource(resource string, err error) {
	if err == nil {
		err = errors.New("source reported a resource failure without a cause")
	}
	in.p.report(Failure{Resource: resource, Scope: ScopeResource, Err: err})
}
