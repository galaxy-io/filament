package pipeline

import (
	"context"
	"errors"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
)

// fakeSink is an in-memory ingestion.Sink. By default it echoes a correct write-side
// CRC; corrupt flips it to force a divergence, and failOn returns an error on the
// nth Apply to exercise the fatal path.
type fakeSink struct {
	mu      sync.Mutex
	written []ingestion.Batch
	applied []ingestion.WritePolicy
	corrupt bool
	failOn  int // 1-based Apply index to fail on; 0 = never
	n       int
}

func (f *fakeSink) Spec() ingestion.SinkSpec                      { return ingestion.SinkSpec{Name: "fake"} }
func (f *fakeSink) Open(context.Context, ingestion.RunSpec) error { return nil }
func (f *fakeSink) Commit(context.Context) error                  { return nil }
func (f *fakeSink) Abort(context.Context) error                   { return nil }
func (f *fakeSink) Name() string                                  { return "fake" }

func (f *fakeSink) Write(_ context.Context, b ingestion.Batch) (ingestion.WriteReceipt, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.written = append(f.written, b)
	crc, nbytes := ingestion.CRC32C(b.Records)
	if f.corrupt {
		crc = ^crc // flip every bit → guaranteed mismatch
	}
	return ingestion.WriteReceipt{URI: "mem://x", Bytes: nbytes, Rows: len(b.Records), WriteCRC: crc}, nil
}

func (f *fakeSink) Apply(ctx context.Context, b ingestion.Batch, opts ingestion.ApplyOptions) (ingestion.WriteReceipt, error) {
	f.mu.Lock()
	f.applied = append(f.applied, opts.Policy)
	f.n++
	fail := f.failOn == f.n
	f.mu.Unlock()
	if fail {
		return ingestion.WriteReceipt{}, errors.New("boom")
	}
	if err := opts.Policy.ValidateRecords(b.Resource, b.Records); err != nil {
		return ingestion.WriteReceipt{}, err
	}
	return f.Write(ctx, b)
}

func (f *fakeSink) batches() []ingestion.Batch {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]ingestion.Batch(nil), f.written...)
}

// collector accumulates emitted facts for assertions.
type collector struct {
	mu  sync.Mutex
	evs []ingestion.Event
}

func (c *collector) emit(e ingestion.Event) {
	c.mu.Lock()
	c.evs = append(c.evs, e)
	c.mu.Unlock()
}

func (c *collector) events() []ingestion.Event {
	c.mu.Lock()
	defer c.mu.Unlock()
	return append([]ingestion.Event(nil), c.evs...)
}

func (c *collector) count(t ingestion.EventType) int {
	n := 0
	for _, e := range c.events() {
		if e.Type == t {
			n++
		}
	}
	return n
}

func rec(resource, id, data string) ingestion.Record {
	return ingestion.NewRecord(resource, id, []byte(data))
}

// run pushes records through a pipeline and blocks for completion. A long flush
// interval keeps the timer out of the way so batching is driven by row count and
// the close-flush, making counts deterministic.
func run(t *testing.T, sink ingestion.Sink, rows int, recs []ingestion.Record) (*collector, error) {
	t.Helper()
	return runWithPolicies(t, sink, rows, recs, defaultWritePolicies(recs))
}

func runWithPolicies(t *testing.T, sink ingestion.Sink, rows int, recs []ingestion.Record, policies map[string]ingestion.WritePolicy) (*collector, error) {
	t.Helper()
	c := &collector{}
	p := New(Config{
		Tenant:        "t1",
		Run:           "r1",
		Sink:          sink,
		Emit:          c.emit,
		WritePolicies: policies,
		Options:       ingestion.RunOptions{BatchMaxRows: rows},
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

func defaultWritePolicies(recs []ingestion.Record) map[string]ingestion.WritePolicy {
	policies := map[string]ingestion.WritePolicy{}
	for _, rec := range recs {
		if _, ok := policies[rec.Resource]; ok {
			continue
		}
		policy := ingestion.WritePolicyForIngestion(ingestion.IngestionSnapshotReplace)
		policy.Resource = rec.Resource
		policies[rec.Resource] = policy
	}
	return policies
}

func TestPipelineHappyPath(t *testing.T) {
	sink := &fakeSink{}
	recs := []ingestion.Record{
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
	if got := c.count(ingestion.EvBatchBuffered); got != 2 {
		t.Errorf("buffered facts = %d, want 2", got)
	}
	if got := c.count(ingestion.EvBatchWritten); got != 2 {
		t.Errorf("written facts = %d, want 2", got)
	}
	if got := c.count(ingestion.EvIntegrityVerified); got != 2 {
		t.Errorf("verified facts = %d, want 2", got)
	}
	if got := c.count(ingestion.EvChunkDivergence); got != 0 {
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
	recs := []ingestion.Record{
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

func TestPipelineDivergenceIsNonFatal(t *testing.T) {
	sink := &fakeSink{corrupt: true}
	recs := []ingestion.Record{rec("users", "1", `{}`), rec("users", "2", `{}`)}
	c, err := run(t, sink, 2, recs)
	if err != nil {
		t.Fatalf("Wait: divergence should not fail the run, got %v", err)
	}
	if got := c.count(ingestion.EvChunkDivergence); got != 1 {
		t.Errorf("divergence facts = %d, want 1", got)
	}
	if got := c.count(ingestion.EvIntegrityVerified); got != 0 {
		t.Errorf("verified facts = %d, want 0 on divergence", got)
	}
	if got := c.count(ingestion.EvBatchWritten); got != 0 {
		t.Errorf("written facts = %d, want 0 on divergence", got)
	}
}

func TestPipelineWriteErrorIsFatal(t *testing.T) {
	sink := &fakeSink{failOn: 1}
	recs := []ingestion.Record{rec("users", "1", `{}`), rec("users", "2", `{}`)}
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
	policy := ingestion.WritePolicyForIngestion(ingestion.IngestionSnapshotUpsert)
	policy.Resource = "users"
	_, err := runWithPolicies(t, sink, 2, []ingestion.Record{rec("users", "1", `{}`)}, map[string]ingestion.WritePolicy{"users": policy})
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got := len(sink.applied); got != 1 {
		t.Fatalf("Apply calls = %d, want 1", got)
	}
	if sink.applied[0].Capability.Mode != ingestion.WriteUpsert {
		t.Fatalf("Apply policy mode = %q, want %q", sink.applied[0].Capability.Mode, ingestion.WriteUpsert)
	}
}

func TestPipelineRejectsLegacyMutationRecords(t *testing.T) {
	sink := &fakeSink{}
	row := rec("users", "1", `{}`)
	row.Op = ingestion.OpUpdate
	_, err := run(t, sink, 2, []ingestion.Record{row})
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
	c, err := run(t, sink, 2, []ingestion.Record{row})
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	var checkpoint *ingestion.CheckpointData
	for _, ev := range c.events() {
		if ev.Type == ingestion.EvBatchWritten {
			checkpoint = ev.Fields.Checkpoint
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

func TestCRC32CDetectsRegrouping(t *testing.T) {
	// Two records, two groupings of the same bytes must not collide — the
	// length-prefix guards reordering/regrouping ambiguity.
	a := []ingestion.Record{rec("r", "1", `{"x":1}`), rec("r", "2", `{"y":2}`)}
	b := []ingestion.Record{rec("r", "12", `{"x":1}`)} // different id/shape
	ca, _ := ingestion.CRC32C(a)
	cb, _ := ingestion.CRC32C(b)
	if ca == cb {
		t.Errorf("CRC collision across distinct record sets: %08x", ca)
	}
	// Identical input is stable.
	ca2, _ := ingestion.CRC32C(a)
	if ca != ca2 {
		t.Errorf("CRC not deterministic: %08x vs %08x", ca, ca2)
	}
}

func assertMonotonicSeq(t *testing.T, evs []ingestion.Event) {
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
