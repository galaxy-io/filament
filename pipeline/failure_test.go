package pipeline

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/events"
)

// scopedSink fails Apply for one resource. With resourceErr it returns a
// *filament.ResourceError. The pipeline learns about isolation from its policy.
type scopedSink struct {
	fakeSink
	failResource string
	failAfter    int // Apply calls for failResource that succeed before failing
	resourceErr  bool
	isolation    bool
	block        chan struct{} // when set, Apply waits on it

	mu           sync.Mutex
	seen         map[string]int
	afterFailure map[string]int // Apply calls per resource after its failure
	failed       atomic.Bool
}

func (s *scopedSink) Spec() filament.SinkSpec {
	return filament.SinkSpec{Name: "scoped"}
}

func (s *scopedSink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if s.block != nil {
		select {
		case <-s.block:
		case <-ctx.Done():
			return filament.WriteReceipt{}, ctx.Err()
		}
	}
	s.mu.Lock()
	if s.seen == nil {
		s.seen = map[string]int{}
		s.afterFailure = map[string]int{}
	}
	if b.Resource == s.failResource && s.failed.Load() {
		s.afterFailure[b.Resource]++
	}
	s.seen[b.Resource]++
	n := s.seen[b.Resource]
	s.mu.Unlock()
	if b.Resource == s.failResource && n > s.failAfter && !s.failed.Load() {
		s.failed.Store(true)
		err := errors.New("permission denied for table")
		if s.resourceErr {
			return filament.WriteReceipt{}, &filament.ResourceError{Resource: b.Resource, Err: err}
		}
		return filament.WriteReceipt{}, err
	}
	return s.fakeSink.Apply(ctx, b, opts)
}

func (s *scopedSink) rowsFor(resource string) int {
	total := 0
	for _, w := range s.batches() {
		if w.resource == resource {
			total += w.rows
		}
	}
	return total
}

func newIsolating(t *testing.T, sink filament.Sink, rows []row, maxRows, writers int, policy FailurePolicy) (*Pipeline, *collector) {
	t.Helper()
	c := &collector{}
	p := New(Config{
		Tenant:        "t1",
		Run:           "r1",
		Sink:          sink,
		Emit:          c.emit,
		WritePolicies: defaultWritePolicies(rows),
		Options:       filament.RunOptions{BatchMaxRows: maxRows, SnapshotParallelism: writers},
		FlushInterval: time.Hour,
		Failures:      policy,
	})
	p.Start(context.Background())
	return p, c
}

func rowsOf(resource string, n int) []row {
	out := make([]row, n)
	for i := range out {
		out[i] = rec(resource, int64(i))
	}
	return out
}

func pushResource(in filament.RecordSink, resource string, n int) error {
	w, err := in.Builder(resource, 0, schema)
	if err != nil {
		return err
	}
	for i := range n {
		w.Int64(int64(i))
		w.String("x")
		if err := w.EndRow(filament.RowMeta{}); err != nil {
			return err
		}
	}
	return nil
}

func TestStrictPolicyKeepsResourceErrorsFatal(t *testing.T) {
	sink := &scopedSink{failResource: "a", resourceErr: true, isolation: true}
	rows := append(rowsOf("a", 4), rowsOf("b", 4)...)
	_, err := run(t, sink, 1, rows)
	if err == nil || !errContains(err, "permission denied") {
		t.Fatalf("strict pipeline err = %v, want the sink failure", err)
	}
}

func TestSourceReportedFailureQuarantinesOnlyThatResource(t *testing.T) {
	sink := &scopedSink{}
	rows := append(rowsOf("a", 4), rowsOf("b", 10)...)
	p, c := newIsolating(t, sink, rows, 2, 1, FailurePolicy{Isolate: true, SourceStopsQuarantined: true})
	in := p.Records()
	reporter, ok := in.(filament.ResourceFailureReporter)
	if !ok {
		t.Fatal("isolating inlet does not implement ResourceFailureReporter")
	}
	reporter.FailResource("a", errors.New("permission denied for table a"))

	_, err := in.Builder("a", 0, schema)
	if !errors.Is(err, filament.ErrResourceQuarantined) {
		t.Fatalf("builder for quarantined resource err = %v, want ErrResourceQuarantined", err)
	}
	var re *filament.ResourceError
	if !errors.As(err, &re) || re.Resource != "a" {
		t.Fatalf("builder error %v is not a ResourceError for a", err)
	}
	if err := pushResource(in, "b", 10); err != nil {
		t.Fatalf("healthy resource push: %v", err)
	}
	p.CloseIngest(nil)
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait = %v, want nil", err)
	}
	if got := sink.rowsFor("b"); got != 10 {
		t.Fatalf("b rows written = %d, want 10", got)
	}
	if got := sink.rowsFor("a"); got != 0 {
		t.Fatalf("a rows written = %d, want 0", got)
	}
	q := p.Quarantined()
	if len(q) != 1 || q["a"].Applied || !errContains(q["a"].Err, "permission denied") {
		t.Fatalf("Quarantined = %+v, want only a, unapplied", q)
	}
	if c.count(events.ResourceQuarantined.Name()) != 1 {
		t.Fatalf("resource.quarantined facts = %d, want 1", c.count(events.ResourceQuarantined.Name()))
	}
}

func TestQuarantineDiscardsUnflushedRows(t *testing.T) {
	sink := &scopedSink{}
	rows := append(rowsOf("a", 3), rowsOf("b", 3)...)
	p, _ := newIsolating(t, sink, rows, 1000, 1, FailurePolicy{Isolate: true})
	in := p.Records()
	// a's rows sit in its builder below the row threshold.
	if err := pushResource(in, "a", 3); err != nil {
		t.Fatal(err)
	}
	if err := pushResource(in, "b", 3); err != nil {
		t.Fatal(err)
	}
	in.(filament.ResourceFailureReporter).FailResource("a", errors.New("source lost a mid-read"))
	p.CloseIngest(nil)
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait = %v", err)
	}
	if sink.rowsFor("a") != 0 || sink.rowsFor("b") != 3 {
		t.Fatalf("rows a=%d b=%d, want a=0 b=3", sink.rowsFor("a"), sink.rowsFor("b"))
	}
}

func TestSinkWithoutIsolationCannotQuarantineAppliedResource(t *testing.T) {
	// A ResourceError from a sink that never declared isolation is a session
	// failure: the sink cannot be trusted to have contained it.
	sink := &scopedSink{failResource: "a", resourceErr: true}
	rows := append(rowsOf("a", 4), rowsOf("b", 4)...)
	p, _ := newIsolating(t, sink, rows, 1, 1, FailurePolicy{Isolate: true})
	err := push(p.Records(), rows)
	p.CloseIngest(err)
	if werr := p.Wait(); werr == nil {
		t.Fatal("Wait = nil, want fatal: the sink cannot isolate")
	}
	if len(p.Quarantined()) != 0 {
		t.Fatalf("Quarantined = %v, want none", p.Quarantined())
	}
}

func TestSourceFailureAfterApplyEscalatesWithoutSinkIsolation(t *testing.T) {
	sink := &scopedSink{}
	rows := append(rowsOf("a", 2), rowsOf("b", 2)...)
	p, _ := newIsolating(t, sink, rows, 1, 1, FailurePolicy{Isolate: true})
	in := p.Records()
	if err := pushResource(in, "a", 2); err != nil {
		t.Fatal(err)
	}
	// Wait until a has reached the sink, so its writes cannot be discarded.
	deadline := time.Now().Add(5 * time.Second)
	for sink.rowsFor("a") == 0 {
		if time.Now().After(deadline) {
			t.Fatal("a never reached the sink")
		}
		time.Sleep(time.Millisecond)
	}
	in.(filament.ResourceFailureReporter).FailResource("a", errors.New("late source failure"))
	err := pushResource(in, "b", 2)
	p.CloseIngest(err)
	werr := p.Wait()
	if werr == nil || !errContains(werr, "cannot discard") {
		t.Fatalf("Wait = %v, want an escalated fatal error", werr)
	}
}

func TestIsolatingSinkQuarantinesWriteFailure(t *testing.T) {
	for _, writers := range []int{1, 4} {
		t.Run(fmt.Sprintf("writers=%d", writers), func(t *testing.T) {
			sink := &scopedSink{failResource: "a", failAfter: 2, resourceErr: true, isolation: true}
			rows := append(rowsOf("a", 50), rowsOf("b", 50)...)
			p, c := newIsolating(t, sink, rows, 1, writers, FailurePolicy{Isolate: true, SinkIsolation: true, SourceStopsQuarantined: true})
			in := p.Records()
			var wg sync.WaitGroup
			var aErr, bErr error
			wg.Add(2)
			go func() { defer wg.Done(); aErr = pushResource(in, "a", 50) }()
			go func() { defer wg.Done(); bErr = pushResource(in, "b", 50) }()
			wg.Wait()
			if !errors.Is(aErr, filament.ErrResourceQuarantined) {
				t.Fatalf("a push err = %v, want ErrResourceQuarantined", aErr)
			}
			if bErr != nil {
				t.Fatalf("b push err = %v, want nil", bErr)
			}
			p.CloseIngest(nil)
			if err := p.Wait(); err != nil {
				t.Fatalf("Wait = %v, want nil", err)
			}
			if got := sink.rowsFor("b"); got != 50 {
				t.Fatalf("b rows = %d, want 50", got)
			}
			q := p.Quarantined()
			if !q["a"].Applied {
				t.Fatalf("Quarantined[a] = %+v, want applied", q["a"])
			}
			// With one writer, nothing for a reaches the sink after its failure.
			// Parallel writers may already be inside Apply; the runner's
			// AbortResource covers those.
			if writers == 1 && sink.afterFailure["a"] != 0 {
				t.Fatalf("a applied %d times after its failure", sink.afterFailure["a"])
			}
			for _, f := range c.events() {
				if f.Name == events.BatchWritten.Name() && f.Resource == "b" {
					return
				}
			}
			t.Fatal("no batch.written for b")
		})
	}
}

func TestIsolatingSinkPlainErrorIsFatal(t *testing.T) {
	sink := &scopedSink{failResource: "a", isolation: true}
	rows := append(rowsOf("a", 4), rowsOf("b", 4)...)
	p, _ := newIsolating(t, sink, rows, 1, 1, FailurePolicy{Isolate: true, SinkIsolation: true})
	err := push(p.Records(), rows)
	p.CloseIngest(err)
	if werr := p.Wait(); werr == nil {
		t.Fatal("Wait = nil, want fatal for an unscoped sink error")
	}
}

func TestIntegrityFailureIsFatalUnderIsolation(t *testing.T) {
	sink := &fakeSink{corrupt: true}
	rows := append(rowsOf("a", 2), rowsOf("b", 2)...)
	p, _ := newIsolating(t, sink, rows, 1, 1, FailurePolicy{Isolate: true, SinkIsolation: true})
	err := push(p.Records(), rows)
	p.CloseIngest(err)
	if werr := p.Wait(); werr == nil || !errContains(werr, "CRC divergence") {
		t.Fatalf("Wait = %v, want CRC divergence", werr)
	}
}

func TestMaxFailedResourcesEscalates(t *testing.T) {
	sink := &scopedSink{}
	rows := append(append(rowsOf("a", 1), rowsOf("b", 1)...), rowsOf("c", 1)...)
	p, _ := newIsolating(t, sink, rows, 1, 1, FailurePolicy{Isolate: true, MaxFailedResources: 1})
	r := p.Records().(filament.ResourceFailureReporter)
	r.FailResource("a", errors.New("a failed"))
	r.FailResource("b", errors.New("b failed"))
	err := pushResource(p.Records(), "c", 1)
	p.CloseIngest(err)
	if werr := p.Wait(); werr == nil || !errContains(werr, "more than the limit of 1") {
		t.Fatalf("Wait = %v, want the failure limit", werr)
	}
}

// A source that does not stop quarantined resources must still see the run
// stop: its writes to a quarantined resource return the fatal error instead
// of being dropped as success.
func TestUnawareSourceSeesFatalErrorOnQuarantinedResource(t *testing.T) {
	sink := &scopedSink{}
	rows := append(rowsOf("a", 1), rowsOf("b", 1)...)
	p, _ := newIsolating(t, sink, rows, 1, 1, FailurePolicy{Isolate: true, MaxFailedResources: 1})
	r := p.Records().(filament.ResourceFailureReporter)
	r.FailResource("a", errors.New("a failed"))
	r.FailResource("b", errors.New("b failed"))
	for _, resource := range []string{"a", "b"} {
		if err := pushResource(p.Records(), resource, 100); err == nil || !errContains(err, "more than the limit of 1") {
			t.Fatalf("push %s = %v, want the failure limit", resource, err)
		}
	}
	p.CloseIngest(nil)
	if werr := p.Wait(); werr == nil || !errContains(werr, "more than the limit of 1") {
		t.Fatalf("Wait = %v, want the failure limit", werr)
	}
}

func TestQuarantineReleasesBuilderBlockedOnBackpressure(t *testing.T) {
	block := make(chan struct{})
	sink := &scopedSink{isolation: true, block: block}
	rows := append(rowsOf("a", 1), rowsOf("b", 1)...)
	p, _ := newIsolating(t, sink, rows, 1, 1, FailurePolicy{Isolate: true, SinkIsolation: true, SourceStopsQuarantined: true})
	in := p.Records()

	aDone := make(chan error, 1)
	go func() {
		w, err := in.Builder("a", 0, schema)
		if err != nil {
			aDone <- err
			return
		}
		for i := 0; ; i++ {
			w.Int64(int64(i))
			w.String("x")
			if err := w.EndRow(filament.RowMeta{}); err != nil {
				aDone <- err
				return
			}
		}
	}()
	time.Sleep(50 * time.Millisecond) // a wedges on the full channel
	in.(filament.ResourceFailureReporter).FailResource("a", errors.New("a failed upstream"))

	select {
	case err := <-aDone:
		if !errors.Is(err, filament.ErrResourceQuarantined) {
			t.Fatalf("a builder err = %v, want ErrResourceQuarantined", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("quarantine did not release a's blocked builder")
	}

	close(block)
	if err := pushResource(in, "b", 5); err != nil {
		t.Fatalf("b push: %v", err)
	}
	p.CloseIngest(nil)
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait = %v", err)
	}
	if got := sink.rowsFor("b"); got != 5 {
		t.Fatalf("b rows = %d, want 5", got)
	}
}

// TestQuarantineRaceManyResources exercises quarantine against parallel writers
// and producers under -race: every healthy resource lands whole.
func TestQuarantineRaceManyResources(t *testing.T) {
	const resources, perResource = 8, 200
	sink := &scopedSink{failResource: "r3", failAfter: 5, resourceErr: true, isolation: true}
	var rows []row
	for i := range resources {
		rows = append(rows, rowsOf(fmt.Sprintf("r%d", i), 1)...)
	}
	p, _ := newIsolating(t, sink, rows, 7, 4, FailurePolicy{Isolate: true, SinkIsolation: true})
	in := p.Records()
	var wg sync.WaitGroup
	errs := make([]error, resources)
	for i := range resources {
		wg.Add(1)
		go func() {
			defer wg.Done()
			errs[i] = pushResource(in, fmt.Sprintf("r%d", i), perResource)
		}()
	}
	wg.Wait()
	for i, err := range errs {
		if i == 3 {
			continue // r3 may finish before its failing batch is applied
		}
		if err != nil {
			t.Fatalf("r%d push err = %v", i, err)
		}
	}
	p.CloseIngest(nil)
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait = %v", err)
	}
	for i := range resources {
		if i == 3 {
			continue
		}
		if got := sink.rowsFor(fmt.Sprintf("r%d", i)); got != perResource {
			t.Fatalf("r%d rows = %d, want %d", i, got, perResource)
		}
	}
	if _, ok := p.Quarantined()["r3"]; !ok {
		t.Fatal("r3 not quarantined")
	}
}

// An unaware source never sees the quarantine: its writes to the failed
// resource succeed and are dropped, so it cannot fail the run by returning
// the error, and every other resource still lands whole.
func TestUnawareSourceKeepsWritingQuarantinedResource(t *testing.T) {
	for _, writers := range []int{1, 4} {
		t.Run(fmt.Sprintf("writers=%d", writers), func(t *testing.T) {
			sink := &scopedSink{failResource: "a", failAfter: 2, resourceErr: true, isolation: true}
			rows := append(rowsOf("a", 1), rowsOf("b", 1)...)
			p, c := newIsolating(t, sink, rows, 1, writers, FailurePolicy{Isolate: true, SinkIsolation: true})
			in := p.Records()
			var wg sync.WaitGroup
			var aErr, bErr error
			wg.Add(2)
			go func() { defer wg.Done(); aErr = pushResource(in, "a", 100) }()
			go func() { defer wg.Done(); bErr = pushResource(in, "b", 100) }()
			wg.Wait()
			if aErr != nil || bErr != nil {
				t.Fatalf("push errs a=%v b=%v, want nil for an unaware source", aErr, bErr)
			}
			p.CloseIngest(nil)
			if err := p.Wait(); err != nil {
				t.Fatalf("Wait = %v", err)
			}
			if got := sink.rowsFor("b"); got != 100 {
				t.Fatalf("b rows = %d, want 100", got)
			}
			if got := sink.rowsFor("a"); got >= 100 {
				t.Fatalf("a rows = %d, want the rows after its failure dropped", got)
			}
			if writers == 1 && sink.afterFailure["a"] != 0 {
				t.Fatalf("a applied %d times after its failure", sink.afterFailure["a"])
			}
			if !p.Quarantined()["a"].Applied {
				t.Fatal("a not quarantined as applied")
			}
			buffered := 0
			for _, f := range c.events() {
				if f.Name == events.BatchBuffered.Name() && f.Resource == "a" {
					buffered++
				}
			}
			if buffered >= 100 {
				t.Fatalf("a buffered %d batches, want dropped batches unreported", buffered)
			}
		})
	}
}

func TestUnawareSourceGetsWorkingBuilderAfterQuarantine(t *testing.T) {
	sink := &scopedSink{}
	rows := append(rowsOf("a", 1), rowsOf("b", 1)...)
	p, _ := newIsolating(t, sink, rows, 2, 1, FailurePolicy{Isolate: true})
	in := p.Records()
	in.(filament.ResourceFailureReporter).FailResource("a", errors.New("a failed"))
	if err := pushResource(in, "a", 10); err != nil {
		t.Fatalf("a push = %v, want nil", err)
	}
	if err := pushResource(in, "b", 10); err != nil {
		t.Fatal(err)
	}
	p.CloseIngest(nil)
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait = %v", err)
	}
	if sink.rowsFor("a") != 0 || sink.rowsFor("b") != 10 {
		t.Fatalf("rows a=%d b=%d, want a=0 b=10", sink.rowsFor("a"), sink.rowsFor("b"))
	}
}

// Dropping must also release an unaware source blocked on backpressure.
func TestUnawareSourceBlockedOnBackpressureIsReleased(t *testing.T) {
	block := make(chan struct{})
	sink := &scopedSink{isolation: true, block: block}
	rows := append(rowsOf("a", 1), rowsOf("b", 1)...)
	p, _ := newIsolating(t, sink, rows, 1, 1, FailurePolicy{Isolate: true, SinkIsolation: true})
	in := p.Records()
	const aRows = 500
	aDone := make(chan error, 1)
	go func() { aDone <- pushResource(in, "a", aRows) }()
	time.Sleep(50 * time.Millisecond) // a wedges on the full channel
	in.(filament.ResourceFailureReporter).FailResource("a", errors.New("a failed upstream"))
	select {
	case err := <-aDone:
		if err != nil {
			t.Fatalf("a push = %v, want nil", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("quarantine did not release the unaware source")
	}
	close(block)
	if err := pushResource(in, "b", 5); err != nil {
		t.Fatal(err)
	}
	p.CloseIngest(nil)
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait = %v", err)
	}
	if sink.rowsFor("b") != 5 {
		t.Fatalf("b rows = %d, want 5", sink.rowsFor("b"))
	}
}
