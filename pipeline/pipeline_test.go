package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
	"github.com/galaxy-io/filament/events"
)

// fakeSink is an in-memory filament.Sink. By default it echoes a correct write-side
// CRC; corrupt flips it to force a divergence, and failOn returns an error on the
// nth Apply to exercise the fatal path.
type fakeSink struct {
	mu      sync.Mutex
	written []written
	applied []filament.WritePolicy
	corrupt bool
	failOn  int // 1-based Apply index to fail on; 0 = never
	n       int
}

// written is what the sink keeps of a batch: rows are released after Apply, so
// the sink copies what it asserts on.
type written struct {
	resource string
	seq      uint64
	rows     int
	ops      []filament.Operation
	cursor   *filament.CheckpointData
}

func (f *fakeSink) Spec() filament.SinkSpec                      { return filament.SinkSpec{Name: "fake"} }
func (f *fakeSink) Open(context.Context, filament.RunSpec) error { return nil }
func (f *fakeSink) Commit(context.Context) error                 { return nil }
func (f *fakeSink) Abort(context.Context) error                  { return nil }
func (f *fakeSink) Name() string                                 { return "fake" }

func (f *fakeSink) Apply(_ context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.applied = append(f.applied, opts.Policy)
	f.n++
	if f.failOn == f.n {
		return filament.WriteReceipt{}, errors.New("boom")
	}
	if err := opts.Policy.ValidateOps(b.Resource, b.Ops); err != nil {
		return filament.WriteReceipt{}, err
	}
	f.written = append(f.written, written{b.Resource, b.Seq, b.NumRows(), b.Ops, b.Cursor})
	crc := batch.CRC(b.Rows, b.Ops)
	if f.corrupt {
		crc = ^crc // flip every bit → guaranteed mismatch
	}
	return filament.WriteReceipt{URI: "mem://x", Bytes: batch.Bytes(b.Rows), Rows: b.NumRows(), WriteCRC: crc}, nil
}

func (f *fakeSink) batches() []written {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]written(nil), f.written...)
}

// collector accumulates emitted facts for assertions.
type collector struct {
	mu  sync.Mutex
	evs []events.Fact
}

func (c *collector) emit(f events.Fact) {
	c.mu.Lock()
	c.evs = append(c.evs, f)
	c.mu.Unlock()
}

func (c *collector) events() []events.Fact {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]events.Fact(nil), c.evs...)
}

func (c *collector) count(name string) int {
	n := 0
	for _, f := range c.events() {
		if f.Name == name {
			n++
		}
	}
	return n
}

var schema = filament.RecordSchema{Fields: []filament.SchemaField{
	{Name: "id", Logical: filament.LogicalInt64},
	{Name: "n", Logical: filament.LogicalString, Nullable: true},
}}

// row is one test row: resource, id, and its meta.
type row struct {
	resource string
	id       int64
	meta     filament.RowMeta
}

func rec(resource string, id int64) row { return row{resource: resource, id: id} }

// push writes rows through the inlet, one builder per resource, and returns the
// first error.
func push(in filament.RecordSink, rows []row) error {
	writers := map[string]filament.RowWriter{}
	for _, r := range rows {
		w, ok := writers[r.resource]
		if !ok {
			var err error
			if w, err = in.Builder(r.resource, 0, schema); err != nil {
				return err
			}
			writers[r.resource] = w
		}
		w.Int64(r.id)
		w.String("x")
		if err := w.EndRow(r.meta); err != nil {
			return err
		}
	}
	return nil
}

// run pushes rows through a pipeline and blocks for completion. A long flush
// interval keeps the timer out of the way so batching is driven by row count and
// the close-flush, making counts deterministic.
func run(t *testing.T, sink filament.Sink, maxRows int, rows []row) (*collector, error) {
	t.Helper()
	return runWithPolicies(t, sink, maxRows, rows, defaultWritePolicies(rows))
}

func runWithPolicies(t *testing.T, sink filament.Sink, maxRows int, rows []row, policies map[string]filament.WritePolicy) (*collector, error) {
	t.Helper()
	c := &collector{}
	p := New(Config{
		Tenant:        "t1",
		Run:           "r1",
		Sink:          sink,
		Emit:          c.emit,
		WritePolicies: policies,
		Options:       filament.RunOptions{BatchMaxRows: maxRows},
		FlushInterval: time.Hour,
	})
	p.Start(context.Background())
	err := push(p.Records(), rows)
	p.CloseIngest(err)
	return c, p.Wait()
}

func defaultWritePolicies(rows []row) map[string]filament.WritePolicy {
	policies := map[string]filament.WritePolicy{}
	for _, r := range rows {
		if _, ok := policies[r.resource]; ok {
			continue
		}
		policy := filament.WritePolicyForIngestion(filament.IngestionFullReplace)
		policy.Resource = r.resource
		policies[r.resource] = policy
	}
	return policies
}

func TestPipelineHappyPath(t *testing.T) {
	sink := &fakeSink{}
	rows := []row{rec("users", 1), rec("users", 2), rec("users", 3)}
	c, err := run(t, sink, 2, rows) // maxRows=2 → batches of [2,1]
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}

	if got := len(sink.batches()); got != 2 {
		t.Fatalf("batches written = %d, want 2", got)
	}
	if got := c.count(events.BatchBuffered.Name()); got != 2 {
		t.Errorf("buffered facts = %d, want 2", got)
	}
	if got := c.count(events.BatchWritten.Name()); got != 2 {
		t.Errorf("written facts = %d, want 2", got)
	}
	if got := c.count(events.IntegrityVerified.Name()); got != 2 {
		t.Errorf("verified facts = %d, want 2", got)
	}
	if got := c.count(events.ChunkDivergence.Name()); got != 0 {
		t.Errorf("divergence facts = %d, want 0", got)
	}

	// Total rows across batches is preserved, and chunk seqs are 0,1.
	var total int
	seqs := map[uint64]bool{}
	for _, b := range sink.batches() {
		total += b.rows
		seqs[b.seq] = true
	}
	if total != 3 {
		t.Errorf("rows across batches = %d, want 3", total)
	}
	if !seqs[0] || !seqs[1] {
		t.Errorf("chunk seqs = %v, want {0,1}", seqs)
	}

	// Fact sequence is monotonic from 1 — the tracker's dedup key.
	assertMonotonicSeq(t, c.events())
}

func TestPipelinePerResourceBatching(t *testing.T) {
	sink := &fakeSink{}
	rows := []row{rec("users", 1), rec("orders", 1), rec("users", 2), rec("orders", 2)}
	// maxRows large + long timer → each resource flushes once at close.
	_, err := run(t, sink, 100, rows)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got := len(sink.batches()); got != 2 {
		t.Fatalf("batches = %d, want 2 (one per resource)", got)
	}
	byRes := map[string]int{}
	for _, b := range sink.batches() {
		byRes[b.resource] += b.rows
		if b.seq != 0 {
			t.Errorf("%s first chunk seq = %d, want 0", b.resource, b.seq)
		}
	}
	if byRes["users"] != 2 || byRes["orders"] != 2 {
		t.Errorf("rows per resource = %v, want users:2 orders:2", byRes)
	}
}

func TestPipelineTimerFlushesPartialChunk(t *testing.T) {
	sink := &fakeSink{}
	c := &collector{}
	p := New(Config{
		Tenant: "t1", Run: "r1", Sink: sink, Emit: c.emit,
		WritePolicies: defaultWritePolicies([]row{rec("users", 0)}),
		Options:       filament.RunOptions{BatchMaxRows: 100},
		FlushInterval: 10 * time.Millisecond,
	})
	p.Start(context.Background())
	w, _ := p.Records().Builder("users", 0, schema)
	w.Int64(1)
	w.String("a")
	if err := w.EndRow(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	time.Sleep(50 * time.Millisecond) // the tick lands; the next EndRow carries the flush
	for i := int64(2); i <= 3; i++ {
		w.Int64(i)
		w.String("b")
		if err := w.EndRow(filament.RowMeta{}); err != nil {
			t.Fatal(err)
		}
	}
	p.CloseIngest(nil)
	if err := p.Wait(); err != nil {
		t.Fatal(err)
	}
	got := sink.batches()
	if len(got) != 2 || got[0].rows != 2 || got[1].rows != 1 {
		t.Fatalf("batches = %+v, want rows [2 1] (timer flush at row 2, close flush)", got)
	}
}

func TestPipelineDivergenceIsFatal(t *testing.T) {
	sink := &fakeSink{corrupt: true}
	c, err := run(t, sink, 2, []row{rec("users", 1), rec("users", 2)})
	if err == nil || !errContains(err, "CRC divergence") {
		t.Fatalf("Wait error = %v, want CRC divergence", err)
	}
	if got := c.count(events.ChunkDivergence.Name()); got != 1 {
		t.Errorf("divergence facts = %d, want 1", got)
	}
	if got := c.count(events.IntegrityVerified.Name()); got != 0 {
		t.Errorf("verified facts = %d, want 0 on divergence", got)
	}
	if got := c.count(events.BatchWritten.Name()); got != 0 {
		t.Errorf("written facts = %d, want 0 on divergence", got)
	}
}

func TestPipelineWriteErrorIsFatal(t *testing.T) {
	sink := &fakeSink{failOn: 1}
	_, err := run(t, sink, 2, []row{rec("users", 1), rec("users", 2)})
	if err == nil {
		t.Fatal("Wait: want error from failed write, got nil")
	}
	if !errContains(err, "boom") {
		t.Errorf("error = %v, want it to wrap \"boom\"", err)
	}
}

func TestPipelineExtractErrorDiscardsPartialRows(t *testing.T) {
	sink := &fakeSink{}
	c := &collector{}
	p := New(Config{
		Tenant: "t1", Run: "r1", Sink: sink, Emit: c.emit,
		WritePolicies: defaultWritePolicies([]row{rec("users", 0)}),
		Options:       filament.RunOptions{BatchMaxRows: 100},
		FlushInterval: time.Hour,
	})
	p.Start(context.Background())
	w, _ := p.Records().Builder("users", 0, schema)
	w.Int64(1) // the source dies inside this row
	p.CloseIngest(errors.New("source died"))
	if err := p.Wait(); err != nil {
		t.Fatalf("Wait: %v (the extract error is the runner's to report)", err)
	}
	if got := len(sink.batches()); got != 0 {
		t.Fatalf("batches = %d, want 0", got)
	}
}

func TestPipelineDispatchesSinkApply(t *testing.T) {
	sink := &fakeSink{}
	policy := filament.WritePolicyForIngestion(filament.IngestionFullUpsert)
	policy.Checkpoint = filament.CheckpointAfterCommit
	policy.Resource = "users"
	c, err := runWithPolicies(t, sink, 2, []row{rec("users", 1)}, map[string]filament.WritePolicy{"users": policy})
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got := len(sink.applied); got != 1 {
		t.Fatalf("Apply calls = %d, want 1", got)
	}
	if sink.applied[0].Capability.Mode != filament.WriteUpsert {
		t.Fatalf("Apply policy mode = %q, want %q", sink.applied[0].Capability.Mode, filament.WriteUpsert)
	}
	for _, fact := range c.events() {
		if written, ok := fact.Data.(events.BatchWrittenEvent); ok && written.CheckpointPolicy != filament.CheckpointAfterCommit {
			t.Fatalf("batch checkpoint policy = %q, want %q", written.CheckpointPolicy, filament.CheckpointAfterCommit)
		}
	}
}

func TestPipelineRejectsMutationsUnderReplace(t *testing.T) {
	sink := &fakeSink{}
	r := rec("users", 1)
	r.meta.Op = filament.OpUpdate
	_, err := run(t, sink, 2, []row{r})
	if err == nil {
		t.Fatal("Wait: want mutation rejection, got nil")
	}
	if !errContains(err, "write policy \"replace\" does not accept update row") {
		t.Fatalf("error = %v, want operation rejection", err)
	}
}

func TestPipelineCursorsFromRowMeta(t *testing.T) {
	sink := &fakeSink{}
	stream := rec("users", 1)
	stream.meta.LSN, stream.meta.Seq = "0/16B6C50", 42
	keyed := rec("orders", 1)
	keyed.meta.Key = []string{"7"}
	coarse := rec("items", 1)
	coarse.meta.Coarse = true
	_, err := run(t, sink, 2, []row{stream, keyed, coarse})
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	for _, b := range sink.batches() {
		switch b.resource {
		case "users":
			if b.cursor.String("lsn") != "0/16B6C50" || b.cursor.Int("seq") != 42 {
				t.Fatalf("stream cursor = %v", b.cursor.Raw())
			}
		case "orders":
			if b.cursor.String("mode") != "keyset" || b.cursor.Int("part") != 0 {
				t.Fatalf("keyset cursor = %v", b.cursor.Raw())
			}
		case "items":
			if b.cursor.Int("ack") != 1 {
				t.Fatalf("coarse cursor = %v", b.cursor.Raw())
			}
		}
	}
}

func TestPipelineDrainPublishesMarker(t *testing.T) {
	sink := &fakeSink{}
	c := &collector{}
	p := New(Config{
		Tenant: "t1", Run: "r1", Sink: sink, Emit: c.emit,
		WritePolicies: defaultWritePolicies([]row{rec("users", 0)}),
		Options:       filament.RunOptions{BatchMaxRows: 100},
		FlushInterval: time.Hour,
	})
	p.Start(context.Background())
	w, _ := p.Records().Builder("users", 3, schema)
	for i := range 2 {
		w.Int64(int64(i))
		w.Null()
		if err := w.EndRow(filament.RowMeta{Coarse: true}); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Drain(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	p.CloseIngest(nil)
	if err := p.Wait(); err != nil {
		t.Fatal(err)
	}
	if got := len(sink.batches()); got != 1 {
		t.Fatalf("sink writes = %d, want 1 (the marker is not written)", got)
	}
	var want int
	for _, f := range c.events() {
		if d, ok := f.Data.(events.BatchWrittenEvent); ok && d.Records == 0 && d.Checkpoint != nil {
			want = d.Checkpoint.Int("want")
		}
	}
	if want != 2 {
		t.Fatalf("marker want = %d, want 2", want)
	}
}

// stuckSink wedges Apply until the pipeline context is cancelled, so
// backpressure fills the channel and blocks the builder.
type stuckSink struct{ fakeSink }

func (s *stuckSink) Apply(ctx context.Context, _ filament.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	<-ctx.Done()
	return filament.WriteReceipt{}, ctx.Err()
}

func TestPipelineCancelReleasesBlockedSource(t *testing.T) {
	sink := &stuckSink{}
	c := &collector{}
	p := New(Config{
		Tenant:        "t1",
		Run:           "r1",
		Sink:          sink,
		Emit:          c.emit,
		WritePolicies: defaultWritePolicies([]row{rec("users", 0)}),
		Options:       filament.RunOptions{BatchMaxRows: 1},
		FlushInterval: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)

	pushed := make(chan error, 1)
	go func() {
		w, _ := p.Records().Builder("users", 0, schema)
		for i := 0; ; i++ {
			w.Int64(int64(i))
			w.Null()
			if err := w.EndRow(filament.RowMeta{}); err != nil {
				pushed <- err
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond) // let the source wedge on a full pipeline
	cancel()

	var perr error
	select {
	case perr = <-pushed:
		if perr == nil {
			t.Fatal("EndRow returned nil after cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("source still blocked 5s after cancel")
	}

	p.CloseIngest(perr)
	waited := make(chan error, 1)
	go func() { waited <- p.Wait() }()
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait still blocked 5s after cancel")
	}
}

func assertMonotonicSeq(t *testing.T, evs []events.Fact) {
	t.Helper()
	var last uint64
	for i, e := range evs {
		if e.Seq <= last {
			t.Errorf("fact %d seq = %d, not greater than previous %d", i, e.Seq, last)
		}
		last = e.Seq
	}
	if len(evs) > 0 && evs[0].Seq != 1 {
		t.Errorf("first fact seq = %d, want 1", evs[0].Seq)
	}
}

func errContains(err error, sub string) bool {
	return err != nil && strings.Contains(err.Error(), sub)
}
