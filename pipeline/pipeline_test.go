package pipeline

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/events"
)

// fakeSink is an in-memory filament.Sink. By default it echoes a correct write-side
// CRC; corrupt flips it to force a divergence, and failOn returns an error on the
// nth Apply to exercise the fatal path.
type fakeSink struct {
	mu      sync.Mutex
	written []filament.Batch
	applied []filament.WritePolicy
	corrupt bool
	failOn  int // 1-based Apply index to fail on; 0 = never
	n       int
}

func (f *fakeSink) Spec() filament.SinkSpec                      { return filament.SinkSpec{Name: "fake"} }
func (f *fakeSink) Open(context.Context, filament.RunSpec) error { return nil }
func (f *fakeSink) Commit(context.Context) error                 { return nil }
func (f *fakeSink) Abort(context.Context) error                  { return nil }
func (f *fakeSink) Name() string                                 { return "fake" }

func (f *fakeSink) Write(_ context.Context, b filament.Batch) (filament.WriteReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, b)
	crc, nbytes := filament.CRC32C(b.Records)
	if f.corrupt {
		crc = ^crc // flip every bit → guaranteed mismatch
	}
	return filament.WriteReceipt{URI: "mem://x", Bytes: nbytes, Rows: len(b.Records), WriteCRC: crc}, nil
}

func (f *fakeSink) Apply(ctx context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	f.mu.Lock()
	f.applied = append(f.applied, opts.Policy)
	f.n++
	fail := f.failOn == f.n
	f.mu.Unlock()
	if fail {
		return filament.WriteReceipt{}, errors.New("boom")
	}
	if err := opts.Policy.ValidateRecords(b.Resource, b.Records); err != nil {
		return filament.WriteReceipt{}, err
	}
	return f.Write(ctx, b)
}

func (f *fakeSink) batches() []filament.Batch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]filament.Batch(nil), f.written...)
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

func rec(resource, id, data string) filament.Record {
	return filament.NewRecord(resource, id, []byte(data))
}

// run pushes records through a pipeline and blocks for completion. A long flush
// interval keeps the timer out of the way so batching is driven by row count and
// the close-flush, making counts deterministic.
func run(t *testing.T, sink filament.Sink, rows int, recs []filament.Record) (*collector, error) {
	t.Helper()
	return runWithPolicies(t, sink, rows, recs, defaultWritePolicies(recs))
}

func runWithPolicies(t *testing.T, sink filament.Sink, rows int, recs []filament.Record, policies map[string]filament.WritePolicy) (*collector, error) {
	t.Helper()
	c := &collector{}
	p := New(Config{
		Tenant:        "t1",
		Run:           "r1",
		Sink:          sink,
		Emit:          c.emit,
		WritePolicies: policies,
		Options:       filament.RunOptions{BatchMaxRows: rows},
		FlushInterval: time.Hour,
	})
	p.Start(context.Background())
	in := p.Records()
	for _, r := range recs {
		if err := in.Push(r); err != nil {
			break // pipeline failed mid-push; Wait reports the cause
		}
	}
	p.CloseIngest()
	return c, p.Wait()
}

func defaultWritePolicies(recs []filament.Record) map[string]filament.WritePolicy {
	policies := map[string]filament.WritePolicy{}
	for _, rec := range recs {
		if _, ok := policies[rec.Resource]; ok {
			continue
		}
		policy := filament.WritePolicyForIngestion(filament.IngestionFullReplace)
		policy.Resource = rec.Resource
		policies[rec.Resource] = policy
	}
	return policies
}

func TestPipelineHappyPath(t *testing.T) {
	sink := &fakeSink{}
	recs := []filament.Record{
		rec("users", "1", `{"n":"a"}`),
		rec("users", "2", `{"n":"b"}`),
		rec("users", "3", `{"n":"c"}`),
	}
	c, err := run(t, sink, 2, recs) // rows=2 → batches of [2,1]
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

	// Total records across batches is preserved, and chunk seqs are 0,1.
	var total int
	seqs := map[uint64]bool{}
	for _, b := range sink.batches() {
		total += len(b.Records)
		seqs[b.Seq] = true
	}
	if total != 3 {
		t.Errorf("records across batches = %d, want 3", total)
	}
	if !seqs[0] || !seqs[1] {
		t.Errorf("chunk seqs = %v, want {0,1}", seqs)
	}

	// Fact sequence is monotonic from 1 — the tracker's dedup key.
	assertMonotonicSeq(t, c.events())
}

func TestPipelinePerResourceBatching(t *testing.T) {
	sink := &fakeSink{}
	recs := []filament.Record{
		rec("users", "1", `{}`),
		rec("orders", "1", `{}`),
		rec("users", "2", `{}`),
		rec("orders", "2", `{}`),
	}
	// rows large + long timer → each resource flushes once at close.
	c, err := run(t, sink, 100, recs)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got := len(sink.batches()); got != 2 {
		t.Fatalf("batches = %d, want 2 (one per resource)", got)
	}
	byRes := map[string]int{}
	for _, b := range sink.batches() {
		byRes[b.Resource] += len(b.Records)
		if b.Seq != 0 {
			t.Errorf("%s first chunk seq = %d, want 0", b.Resource, b.Seq)
		}
	}
	if byRes["users"] != 2 || byRes["orders"] != 2 {
		t.Errorf("records per resource = %v, want users:2 orders:2", byRes)
	}
	_ = c
}

func TestPipelineDivergenceIsFatal(t *testing.T) {
	sink := &fakeSink{corrupt: true}
	recs := []filament.Record{rec("users", "1", `{}`), rec("users", "2", `{}`)}
	c, err := run(t, sink, 2, recs)
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
	recs := []filament.Record{rec("users", "1", `{}`), rec("users", "2", `{}`)}
	_, err := run(t, sink, 2, recs)
	if err == nil {
		t.Fatal("Wait: want error from failed write, got nil")
	}
	if !errContains(err, "boom") {
		t.Errorf("error = %v, want it to wrap \"boom\"", err)
	}
}

func TestPipelineDispatchesSinkApply(t *testing.T) {
	sink := &fakeSink{}
	policy := filament.WritePolicyForIngestion(filament.IngestionFullUpsert)
	policy.Checkpoint = filament.CheckpointAfterCommit
	policy.Resource = "users"
	c, err := runWithPolicies(t, sink, 2, []filament.Record{rec("users", "1", `{}`)}, map[string]filament.WritePolicy{"users": policy})
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

func TestPipelineRejectsLegacyMutationRecords(t *testing.T) {
	sink := &fakeSink{}
	row := rec("users", "1", `{}`)
	row.Op = filament.OpUpdate
	_, err := run(t, sink, 2, []filament.Record{row})
	if err == nil {
		t.Fatal("Wait: want legacy mutation rejection, got nil")
	}
	if !errContains(err, "write policy \"replace\" does not accept update record") {
		t.Fatalf("error = %v, want operation rejection", err)
	}
}

func TestPipelinePublishesLSNCheckpointFromRecordMeta(t *testing.T) {
	sink := &fakeSink{}
	row := rec("users", "1", `{}`)
	row.Meta.LSN = "0/16B6C50"
	row.Meta.Seq = 42
	c, err := run(t, sink, 2, []filament.Record{row})
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	var checkpoint *filament.CheckpointData
	for _, f := range c.events() {
		if d, ok := f.Data.(events.BatchWrittenEvent); ok {
			checkpoint = d.Checkpoint
			break
		}
	}
	if checkpoint == nil {
		t.Fatal("batch written checkpoint is nil")
	}
	if got := checkpoint.String("lsn"); got != "0/16B6C50" {
		t.Fatalf("checkpoint lsn = %q, want 0/16B6C50", got)
	}
	if got := checkpoint.Int("seq"); got != 42 {
		t.Fatalf("checkpoint seq = %d, want 42", got)
	}
}

// stuckSink wedges Apply until the pipeline context is cancelled, so
// backpressure fills the channels and blocks the inlet.
type stuckSink struct{ fakeSink }

func (s *stuckSink) Apply(ctx context.Context, _ filament.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	<-ctx.Done()
	return filament.WriteReceipt{}, ctx.Err()
}

func TestPipelineCancelReleasesBlockedPush(t *testing.T) {
	sink := &stuckSink{}
	c := &collector{}
	p := New(Config{
		Tenant:        "t1",
		Run:           "r1",
		Sink:          sink,
		Emit:          c.emit,
		WritePolicies: defaultWritePolicies([]filament.Record{rec("users", "0", `{}`)}),
		Options:       filament.RunOptions{BatchMaxRows: 1},
		FlushInterval: time.Hour,
	})
	ctx, cancel := context.WithCancel(context.Background())
	p.Start(ctx)

	pushed := make(chan error, 1)
	go func() {
		in := p.Records()
		for i := 0; ; i++ {
			if err := in.Push(rec("users", strconv.Itoa(i), `{}`)); err != nil {
				pushed <- err
				return
			}
		}
	}()

	time.Sleep(50 * time.Millisecond) // let the source wedge on a full pipeline
	cancel()

	select {
	case err := <-pushed:
		if err == nil {
			t.Fatal("Push returned nil after cancel")
		}
	case <-time.After(5 * time.Second):
		t.Fatal("Push still blocked 5s after cancel")
	}

	p.CloseIngest()
	waited := make(chan error, 1)
	go func() { waited <- p.Wait() }()
	select {
	case <-waited:
	case <-time.After(5 * time.Second):
		t.Fatal("Wait still blocked 5s after cancel")
	}
}

func TestCRC32CDetectsRegrouping(t *testing.T) {
	// Two records, two groupings of the same bytes must not collide — the
	// length-prefix guards reordering/regrouping ambiguity.
	a := []filament.Record{rec("r", "1", `{"x":1}`), rec("r", "2", `{"y":2}`)}
	b := []filament.Record{rec("r", "12", `{"x":1}`)} // different id/shape
	ca, _ := filament.CRC32C(a)
	cb, _ := filament.CRC32C(b)
	if ca == cb {
		t.Errorf("CRC collision across distinct record sets: %08x", ca)
	}
	// Identical input is stable.
	ca2, _ := filament.CRC32C(a)
	if ca != ca2 {
		t.Errorf("CRC not deterministic: %08x vs %08x", ca, ca2)
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
