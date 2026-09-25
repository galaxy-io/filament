package pipeline

import (
	"context"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/events"
	"github.com/galaxy-io/filament/transform"
)

// renameAndSet renames n → name and overwrites it with a literal, so a batch
// that went through the plan is distinguishable by both schema and value.
var renameAndSet = &transform.Definition{Version: 1, Resources: map[string]transform.Resource{
	"users": {Steps: []transform.Step{
		{Rename: map[string]string{"n": "name"}},
		{Compute: map[string]transform.Expr{"name": {Lit: "hello"}}},
	}},
}}

// columnCaptureSink keeps each resource's column names and the first row of
// its last string column, since batch rows are released after Apply.
type columnCaptureSink struct {
	fakeSink
	cmu    sync.Mutex
	fields map[string][]string
	value  map[string]string
}

func (s *columnCaptureSink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	rows := b.Rows()
	s.cmu.Lock()
	if s.fields == nil {
		s.fields, s.value = map[string][]string{}, map[string]string{}
	}
	var names []string
	for _, f := range rows.Schema().Fields() {
		names = append(names, f.Name)
	}
	s.fields[b.Resource] = names
	s.value[b.Resource] = rows.Column(1).(*array.String).Value(0)
	s.cmu.Unlock()
	return s.fakeSink.Apply(ctx, b, opts)
}

func runTransform(t *testing.T, sink filament.Sink, def *transform.Definition, alloc memory.Allocator, maxRows int, rows []row) (*collector, error) {
	t.Helper()
	c := &collector{}
	p := New(Config{
		Tenant: "t1", Run: "r1", Sink: sink, Emit: c.emit,
		WritePolicies: defaultWritePolicies(rows),
		Options:       filament.RunOptions{BatchMaxRows: maxRows},
		FlushInterval: time.Hour,
		Allocator:     alloc,
		Transform:     def,
	})
	p.Start(context.Background())
	err := push(p.Records(), rows)
	p.CloseIngest(err)
	return c, p.Wait()
}

func TestPipelineTransformsNamedResourceOnly(t *testing.T) {
	sink := &columnCaptureSink{}
	rows := []row{rec("users", 1), rec("users", 2), rec("orders", 1)}
	c, err := runTransform(t, sink, renameAndSet, nil, 10, rows)
	if err != nil {
		t.Fatalf("Wait: %v", err)
	}
	if got := strings.Join(sink.fields["users"], ","); got != "id,name" {
		t.Errorf("users columns = %q, want id,name", got)
	}
	if got := sink.value["users"]; got != "hello" {
		t.Errorf("users name = %q, want hello", got)
	}
	if got := strings.Join(sink.fields["orders"], ","); got != "id,n" {
		t.Errorf("orders columns = %q, want id,n (bypass)", got)
	}
	if got := sink.value["orders"]; got != "x" {
		t.Errorf("orders n = %q, want x (bypass)", got)
	}
	if got := c.count(events.IntegrityVerified.Name()); got != 2 {
		t.Errorf("verified facts = %d, want 2", got)
	}
}

func TestPipelineTransformKeepsMarkerBehindRows(t *testing.T) {
	sink := &fakeSink{}
	c := &collector{}
	p := New(Config{
		Tenant: "t1", Run: "r1", Sink: sink, Emit: c.emit,
		WritePolicies: defaultWritePolicies([]row{rec("users", 0)}),
		Options:       filament.RunOptions{BatchMaxRows: 1},
		FlushInterval: time.Hour,
		Transform:     renameAndSet,
	})
	p.Start(context.Background())
	w, err := p.Records().Builder("users", 3, schema)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 8 {
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
	if got := len(sink.batches()); got != 8 {
		t.Fatalf("sink writes = %d, want 8", got)
	}
	var written, markerAt int
	for _, f := range c.events() {
		d, ok := f.Data.(events.BatchWrittenEvent)
		if !ok {
			continue
		}
		if d.Records == 0 {
			markerAt = written
			continue
		}
		written++
	}
	if markerAt != 8 {
		t.Fatalf("marker written after %d row batches, want 8", markerAt)
	}
}

func TestPipelineTransformErrorReleasesBatches(t *testing.T) {
	alloc := memory.NewCheckedAllocator(memory.DefaultAllocator)
	failing := &transform.Definition{Version: 1, Resources: map[string]transform.Resource{
		"users": {Steps: []transform.Step{
			{Compute: map[string]transform.Expr{"d": {Fn: "to_date", Args: []transform.Expr{{Col: "n"}}}}},
		}},
	}}
	sink := &fakeSink{}
	p := New(Config{
		Sink: sink, Allocator: alloc,
		WritePolicies: defaultWritePolicies([]row{rec("users", 0)}),
		Options:       filament.RunOptions{BatchMaxRows: 1},
		FlushInterval: time.Hour,
		Transform:     failing,
	})
	p.Start(context.Background())
	w, err := p.Records().Builder("users", 0, schema)
	if err != nil {
		t.Fatal(err)
	}
	for i := range 20 {
		w.Int64(int64(i))
		w.String("not a date")
		if err = w.EndRow(filament.RowMeta{}); err != nil {
			break
		}
	}
	p.CloseIngest(err)
	if err := p.Wait(); err == nil || !strings.Contains(err.Error(), "transform users seq") {
		t.Fatalf("Wait = %v, want transform error", err)
	}
	if got := len(sink.batches()); got != 0 {
		t.Fatalf("sink writes = %d, want 0", got)
	}
	alloc.AssertSize(t, 0)
}

func TestPipelineTransformCompileErrorFailsBuilder(t *testing.T) {
	missing := &transform.Definition{Version: 1, Resources: map[string]transform.Resource{
		"users": {Steps: []transform.Step{{Drop: []string{"nope"}}}},
	}}
	p := New(Config{
		Sink:          &fakeSink{},
		WritePolicies: defaultWritePolicies([]row{rec("users", 0)}),
		Transform:     missing,
	})
	if _, err := p.Records().Builder("users", 0, schema); err == nil || !strings.Contains(err.Error(), "nope") {
		t.Fatalf("Builder = %v, want compile error naming the column", err)
	}
}
