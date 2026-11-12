package runner

import (
	"context"
	"errors"
	"slices"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/datastore/sqlite"
	"github.com/galaxy-io/filament/eventbus"
	"github.com/galaxy-io/filament/eventbus/inproc"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/pipeline"
	"github.com/galaxy-io/filament/registry"
)

// isoSource extracts every resource but "bad". For "bad" it reports a
// permission failure through FailResource when the run isolates failures,
// and otherwise returns it, as an unmigrated source would. With
// returnAlways it returns the error even under isolation.
type isoSource struct {
	commitTestSource
	failBad      bool
	returnAlways bool
	// naive returns every builder error, as a source written before
	// isolation does. It does not implement ResourceIsolatingSource.
	naive bool
	rows  int
}

func (s *isoSource) Extract(_ context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
	denied := errors.New("permission denied for table bad")
	for _, resource := range opts.Resources {
		if resource == "bad" && s.failBad {
			reporter, ok := sink.(filament.ResourceFailureReporter)
			if !ok || s.returnAlways {
				return denied
			}
			reporter.FailResource(resource, denied)
			continue
		}
		schema := filament.RecordSchema{Resource: resource, Fields: []filament.SchemaField{{Name: "id", Logical: filament.LogicalInt64}}}
		w, err := sink.Builder(resource, 0, schema)
		if err != nil {
			var re *filament.ResourceError
			if !s.naive && errors.As(err, &re) {
				continue // quarantined by the writer: stop this resource only
			}
			return err
		}
		for i := range s.rows {
			w.Int64(int64(i))
			if err := w.EndRow(filament.RowMeta{}); err != nil {
				var re *filament.ResourceError
				if !s.naive && errors.As(err, &re) {
					break
				}
				return err
			}
		}
	}
	return nil
}

// isoSink records writes per resource. It fails Apply for failApply, scoped
// to that resource when isolation is advertised.
type isoSink struct {
	incrementalTestSink
	isolation  bool
	failApply  string
	abortErr   error
	mu         sync.Mutex
	rows       map[string]int
	aborted    []string
	commits    int
	sinkAborts int
}

func (s *isoSink) Spec() filament.SinkSpec {
	return s.incrementalTestSink.Spec()
}

func (s *isoSink) IsolatesResources() bool { return s.isolation }

func (s *isoSink) Apply(_ context.Context, b *arrowbatch.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if b.Resource == s.failApply {
		err := errors.New("column type mismatch")
		if s.isolation {
			return filament.WriteReceipt{}, &filament.ResourceError{Resource: b.Resource, Err: err}
		}
		return filament.WriteReceipt{}, err
	}
	if s.rows == nil {
		s.rows = map[string]int{}
	}
	s.rows[b.Resource] += b.NumRows()
	return filament.WriteReceipt{Rows: b.NumRows(), WriteCRC: b.IntegrityCRC()}, nil
}

func (s *isoSink) AbortResource(_ context.Context, resource string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.aborted = append(s.aborted, resource)
	delete(s.rows, resource)
	return s.abortErr
}

func (s *isoSink) Commit(context.Context) error { s.mu.Lock(); s.commits++; s.mu.Unlock(); return nil }

func (s *isoSink) Abort(context.Context) error {
	s.mu.Lock()
	s.sinkAborts++
	s.mu.Unlock()
	return nil
}
func (*isoSink) Begin(context.Context) error { return nil }
func (*isoSink) Close(context.Context) error { return nil }

type isoOutcome struct {
	facts []events.Fact
}

func (o isoOutcome) terminal(t *testing.T) events.Fact {
	t.Helper()
	for _, f := range o.facts {
		switch f.Data.(type) {
		case events.RunCompletedEvent, events.RunFailedEvent, events.RunPartialEvent:
			return f
		}
	}
	t.Fatal("no run terminal fact")
	return events.Fact{}
}

func (o isoOutcome) resourceFailed(resource string) (events.ResourceFailedEvent, bool) {
	for _, f := range o.facts {
		if d, ok := f.Data.(events.ResourceFailedEvent); ok && f.Resource == resource {
			return d, true
		}
	}
	return events.ResourceFailedEvent{}, false
}

func (o isoOutcome) resourceCompleted(resource string) bool {
	for _, f := range o.facts {
		if _, ok := f.Data.(events.ResourceCompletedEvent); ok && f.Resource == resource {
			return true
		}
	}
	return false
}

func runIsolated(t *testing.T, src filament.Source, sink *isoSink, isolate bool, resources ...string) isoOutcome {
	t.Helper()
	bus := inproc.New()
	defer func() { _ = bus.Close() }()
	sub, err := bus.Subscribe(events.AllPattern(), eventbus.SubOpts{})
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = sub.Close() }()
	var mu sync.Mutex
	var facts []events.Fact
	done := make(chan struct{})
	go func() {
		defer close(done)
		for msg := range sub.C() {
			f, decodeErr := events.Decode(msg)
			_ = msg.Ack()
			if decodeErr != nil {
				continue
			}
			mu.Lock()
			facts = append(facts, f)
			mu.Unlock()
			switch f.Data.(type) {
			case events.RunCompletedEvent, events.RunFailedEvent, events.RunPartialEvent:
				return
			}
		}
	}()
	sources, sinks := registry.NewSources(), registry.NewSinks()
	sources.Register("iso", func() filament.Source { return src })
	sinks.Register("iso-sink", func() filament.Sink { return sink })
	if err := RunOne(context.Background(), Deps{Bus: bus, DataStore: sqlite.NewMemory(), Sources: sources, Sinks: sinks}, filament.RunSpec{
		Tenant: "t1", Run: "iso", Source: filament.Ref{Connector: "iso"}, Sink: filament.Ref{Connector: "iso-sink"},
		Resources:      resources,
		IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionFullReplace},
		Options:        filament.RunOptions{IsolateResourceFailures: isolate, BatchMaxRows: 2},
	}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(5 * time.Second):
		t.Fatal("no terminal fact within 5s")
	}
	mu.Lock()
	defer mu.Unlock()
	return isoOutcome{facts: slices.Clone(facts)}
}

func TestIsolatedSourceFailureCommitsHealthyResources(t *testing.T) {
	src, sink := &isoSource{failBad: true, rows: 5}, &isoSink{}
	out := runIsolated(t, src, sink, true, "good", "bad", "also_good")
	done, ok := out.terminal(t).Data.(events.RunCompletedEvent)
	if !ok {
		t.Fatalf("terminal = %T, want run.completed", out.terminal(t).Data)
	}
	if !slices.Equal(done.FailedResources, []string{"bad"}) || done.Records != 10 {
		t.Fatalf("run.completed = %+v, want bad failed and 10 records", done)
	}
	if sink.commits != 1 || sink.sinkAborts != 0 || len(sink.aborted) != 0 {
		t.Fatalf("sink commits=%d aborts=%d aborted=%v", sink.commits, sink.sinkAborts, sink.aborted)
	}
	if sink.rows["good"] != 5 || sink.rows["also_good"] != 5 || sink.rows["bad"] != 0 {
		t.Fatalf("rows = %v", sink.rows)
	}
	// A failed replacement is truncated or never promoted, so it is discarded.
	failed, ok := out.resourceFailed("bad")
	if !ok || failed.Error == "" || !failed.Discarded {
		t.Fatalf("resource.failed(bad) = %+v, %v", failed, ok)
	}
	if !out.resourceCompleted("good") || out.resourceCompleted("bad") {
		t.Fatal("wrong resource.completed facts")
	}
}

func TestStrictRunStillFailsOnSourceResourceError(t *testing.T) {
	src, sink := &isoSource{failBad: true, rows: 5}, &isoSink{}
	out := runIsolated(t, src, sink, false, "good", "bad")
	if _, ok := out.terminal(t).Data.(events.RunFailedEvent); !ok {
		t.Fatalf("terminal = %T, want run.failed", out.terminal(t).Data)
	}
	if sink.commits != 0 || sink.sinkAborts != 1 {
		t.Fatalf("sink commits=%d aborts=%d, want 0 and 1", sink.commits, sink.sinkAborts)
	}
}

// A source that returns its error has stopped; nothing proves the other
// resources finished, so isolation must not commit them.
func TestIsolatedRunFailsWhenSourceReturnsError(t *testing.T) {
	src, sink := &isoSource{failBad: true, returnAlways: true, rows: 5}, &isoSink{}
	out := runIsolated(t, src, sink, true, "good", "bad", "later")
	if _, ok := out.terminal(t).Data.(events.RunFailedEvent); !ok {
		t.Fatalf("terminal = %T, want run.failed", out.terminal(t).Data)
	}
	if sink.commits != 0 {
		t.Fatal("sink committed after the source returned an error")
	}
}

func TestIsolatedSinkFailureAbortsOnlyThatResource(t *testing.T) {
	src, sink := &isoSource{rows: 6}, &isoSink{isolation: true, failApply: "bad"}
	out := runIsolated(t, src, sink, true, "good", "bad")
	done, ok := out.terminal(t).Data.(events.RunCompletedEvent)
	if !ok || !slices.Equal(done.FailedResources, []string{"bad"}) {
		t.Fatalf("terminal = %+v, want run.completed with bad failed", out.terminal(t).Data)
	}
	if !slices.Equal(sink.aborted, []string{"bad"}) || sink.commits != 1 || sink.sinkAborts != 0 {
		t.Fatalf("aborted=%v commits=%d sinkAborts=%d", sink.aborted, sink.commits, sink.sinkAborts)
	}
	if sink.rows["good"] != 6 {
		t.Fatalf("good rows = %d, want 6", sink.rows["good"])
	}
}

func TestSinkWithoutIsolationFailsTheRun(t *testing.T) {
	src, sink := &isoSource{rows: 6}, &isoSink{failApply: "bad"}
	out := runIsolated(t, src, sink, true, "good", "bad")
	if _, ok := out.terminal(t).Data.(events.RunFailedEvent); !ok {
		t.Fatalf("terminal = %T, want run.failed", out.terminal(t).Data)
	}
	if sink.commits != 0 || sink.sinkAborts != 1 {
		t.Fatalf("commits=%d sinkAborts=%d", sink.commits, sink.sinkAborts)
	}
}

func TestEveryResourceFailedFailsTheRun(t *testing.T) {
	src, sink := &isoSource{failBad: true, rows: 5}, &isoSink{}
	out := runIsolated(t, src, sink, true, "bad")
	if _, ok := out.terminal(t).Data.(events.RunFailedEvent); !ok {
		t.Fatalf("terminal = %T, want run.failed", out.terminal(t).Data)
	}
	if sink.commits != 0 {
		t.Fatal("sink committed a run where every resource failed")
	}
}

func TestResourceAbortFailureFailsTheRun(t *testing.T) {
	src := &isoSource{rows: 6}
	sink := &isoSink{isolation: true, failApply: "bad", abortErr: errors.New("stage drop timed out")}
	out := runIsolated(t, src, sink, true, "good", "bad")
	if _, ok := out.terminal(t).Data.(events.RunFailedEvent); !ok {
		t.Fatalf("terminal = %T, want run.failed", out.terminal(t).Data)
	}
	if sink.commits != 0 || sink.sinkAborts != 1 {
		t.Fatalf("commits=%d sinkAborts=%d", sink.commits, sink.sinkAborts)
	}
}

// awareSource stops a quarantined resource itself, so it is told about it.
type awareSource struct{ isoSource }

func (*awareSource) IsolatesResourceFailures() bool { return true }

// A source written before isolation still gets sink-side isolation: the
// quarantined resource's rows are dropped instead of failing its builder.
func TestUnchangedSourceGetsSinkIsolation(t *testing.T) {
	src, sink := &isoSource{naive: true, rows: 40}, &isoSink{isolation: true, failApply: "bad"}
	out := runIsolated(t, src, sink, true, "good", "bad", "later")
	done, ok := out.terminal(t).Data.(events.RunCompletedEvent)
	if !ok || !slices.Equal(done.FailedResources, []string{"bad"}) {
		t.Fatalf("terminal = %+v, want run.completed with bad failed", out.terminal(t).Data)
	}
	if sink.rows["good"] != 40 || sink.rows["later"] != 40 {
		t.Fatalf("rows = %v, want good and later whole", sink.rows)
	}
	if !slices.Equal(sink.aborted, []string{"bad"}) || sink.commits != 1 {
		t.Fatalf("aborted=%v commits=%d", sink.aborted, sink.commits)
	}
}

func TestAwareSourceStopsQuarantinedResource(t *testing.T) {
	src := &awareSource{isoSource{naive: true, rows: 40}}
	sink := &isoSink{isolation: true, failApply: "bad"}
	out := runIsolated(t, src, sink, true, "good", "bad")
	// A naive loop that is told about the quarantine returns the error, which
	// fails the run: the marker is a promise the source must keep.
	if _, ok := out.terminal(t).Data.(events.RunFailedEvent); !ok {
		t.Fatalf("terminal = %T, want run.failed for a source that breaks its promise", out.terminal(t).Data)
	}

	src = &awareSource{isoSource{rows: 40}}
	sink = &isoSink{isolation: true, failApply: "bad"}
	out = runIsolated(t, src, sink, true, "good", "bad")
	if _, ok := out.terminal(t).Data.(events.RunCompletedEvent); !ok {
		t.Fatalf("terminal = %T, want run.completed", out.terminal(t).Data)
	}
}

// A resource that failed before its first write is still aborted on an
// isolating sink, so a commit-time replacement is never published empty.
func TestIsolatingSinkAbortsUnwrittenFailedResource(t *testing.T) {
	src, sink := &isoSource{failBad: true, rows: 5}, &isoSink{isolation: true}
	out := runIsolated(t, src, sink, true, "good", "bad")
	if _, ok := out.terminal(t).Data.(events.RunCompletedEvent); !ok {
		t.Fatalf("terminal = %T, want run.completed", out.terminal(t).Data)
	}
	if !slices.Equal(sink.aborted, []string{"bad"}) {
		t.Fatalf("aborted = %v, want [bad] even though bad never reached Apply", sink.aborted)
	}
}

func settleSpec() filament.RunSpec {
	return filament.RunSpec{Tenant: "t1", Run: "settle", Sink: filament.Ref{Connector: "iso-sink"}, WritePolicies: map[string]filament.WritePolicy{
		"orders":   {DestinationResource: "sales_orders"},
		"invoices": {DestinationResource: "billing"},
		"refunds":  {DestinationResource: "billing"},
	}}
}

func TestSettleAbortsByDestinationName(t *testing.T) {
	bus := inproc.New()
	defer func() { _ = bus.Close() }()
	sink := &isoSink{isolation: true}
	em := newEmitter(context.Background(), bus, nil, "t1", "settle")
	quarantined := map[string]pipeline.Quarantine{"orders": {Err: errors.New("denied"), Applied: true}}
	if err := settleQuarantine(context.Background(), Deps{}, settleSpec(), sink, em, []string{"orders", "users"}, quarantined); err != nil {
		t.Fatal(err)
	}
	if !slices.Equal(sink.aborted, []string{"sales_orders"}) {
		t.Fatalf("aborted = %v, want the destination name sales_orders", sink.aborted)
	}
}

func TestSettleRefusesSharedDestination(t *testing.T) {
	bus := inproc.New()
	defer func() { _ = bus.Close() }()
	sink := &isoSink{isolation: true}
	em := newEmitter(context.Background(), bus, nil, "t1", "settle")
	quarantined := map[string]pipeline.Quarantine{"invoices": {Err: errors.New("denied"), Applied: true}}
	err := settleQuarantine(context.Background(), Deps{}, settleSpec(), sink, em, []string{"invoices", "refunds"}, quarantined)
	if err == nil || !strings.Contains(err.Error(), "shares destination") {
		t.Fatalf("settle = %v, want a shared-destination failure", err)
	}
	if len(sink.aborted) != 0 {
		t.Fatalf("aborted = %v, want nothing aborted", sink.aborted)
	}
}
