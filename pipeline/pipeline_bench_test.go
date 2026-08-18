package pipeline

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
)

// nullSink is a zero-I/O filament.Sink. It recomputes the write-side CRC exactly as a
// real sink does (so the integrity work stays in the measurement) and discards
// the batch — it stores nothing, so memory stays flat across bench iterations.
type nullSink struct{}

func (nullSink) Spec() filament.SinkSpec                      { return filament.SinkSpec{Name: "null"} }
func (nullSink) Name() string                                 { return "null" }
func (nullSink) Open(context.Context, filament.RunSpec) error { return nil }
func (nullSink) Commit(context.Context) error                 { return nil }
func (nullSink) Abort(context.Context) error                  { return nil }

func (nullSink) Apply(_ context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if err := opts.Policy.ValidateOps(b.Resource, b.Ops); err != nil {
		return filament.WriteReceipt{}, err
	}
	return filament.WriteReceipt{WriteCRC: batch.CRC(b.Rows, b.Ops), Bytes: batch.Bytes(b.Rows), Rows: b.NumRows()}, nil
}

var benchSchema = filament.RecordSchema{Fields: []filament.SchemaField{
	{Name: "i", Logical: filament.LogicalInt64},
	{Name: "name", Logical: filament.LogicalString},
	{Name: "payload", Logical: filament.LogicalString},
}}

const benchPayload = "xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"

// benchRow appends one ~50-byte row.
func benchRow(w filament.RowWriter, i int) error {
	w.Int64(int64(i))
	w.String("row")
	w.String(benchPayload)
	return w.EndRow(filament.RowMeta{})
}

// BenchmarkNullSinkPipeline is the top rung of the subtraction ladder: rows
// appended through the full builder→writer→integrity loop into a zero-I/O sink,
// no source read, bus, or storage. allocs/op ÷ rows is the pipeline's own floor —
// whatever the engine bench costs above this is source read + bus + tracker.
func BenchmarkNullSinkPipeline(b *testing.B) {
	const rows = 20_000
	ctx := context.Background()

	b.ReportAllocs()
	var processed int64
	for b.Loop() {
		p := New(Config{
			Run:           "bench",
			Sink:          nullSink{},
			WritePolicies: map[string]filament.WritePolicy{"bench": appendPolicy("bench")},
			Options:       filament.RunOptions{BatchMaxRows: 1000},
			FlushInterval: time.Hour, // row-count + close drive batching, not the timer
		})
		p.Start(ctx)
		w, err := p.Records().Builder("bench", 0, benchSchema)
		if err != nil {
			b.Fatal(err)
		}
		for i := range rows {
			if err := benchRow(w, i); err != nil {
				b.Fatalf("row: %v", err)
			}
		}
		p.CloseIngest(nil)
		if err := p.Wait(); err != nil {
			b.Fatalf("wait: %v", err)
		}
		processed += rows
	}
	if s := b.Elapsed().Seconds(); s > 0 {
		b.ReportMetric(float64(processed)/s, "rows/sec")
	}
}

func appendPolicy(resource string) filament.WritePolicy {
	policy := filament.WritePolicyForIngestion(filament.IngestionFullAppend)
	policy.Resource = resource
	return policy
}

// BenchmarkNullSinkPipelineParallel is the Docker-free ceiling with parallelism:
// one builder per resource, each fed from its own goroutine (mimicking a parallel
// source), and a matching writer pool. Sweeping parallelism shows the pipeline
// itself scaling once building and the read CRC are no longer single-goroutine —
// isolating that from the shared-database contention the macro engine benches hit.
func BenchmarkNullSinkPipelineParallel(b *testing.B) {
	const rows = 40_000
	const resources = 8
	ctx := context.Background()

	for _, parallelism := range []int{1, 4, 8} {
		b.Run(fmt.Sprintf("parallelism=%d", parallelism), func(b *testing.B) {
			b.ReportAllocs()
			var processed int64
			for b.Loop() {
				pl := New(Config{
					Run:           "bench",
					Sink:          nullSink{},
					WritePolicies: appendPolicies(resources),
					Options:       filament.RunOptions{BatchMaxRows: 1000, SnapshotParallelism: parallelism},
					FlushInterval: time.Hour,
				})
				pl.Start(ctx)
				in := pl.Records()

				var wg sync.WaitGroup
				for r := range resources {
					wg.Add(1)
					go func(r int) {
						defer wg.Done()
						w, err := in.Builder("res-"+strconv.Itoa(r), 0, benchSchema)
						if err != nil {
							return
						}
						for i := range rows / resources {
							if err := benchRow(w, i); err != nil {
								return // pipeline failed; Wait surfaces it
							}
						}
					}(r)
				}
				wg.Wait()
				pl.CloseIngest(nil)
				if err := pl.Wait(); err != nil {
					b.Fatalf("wait: %v", err)
				}
				processed += rows
			}
			if s := b.Elapsed().Seconds(); s > 0 {
				b.ReportMetric(float64(processed)/s, "rows/sec")
			}
		})
	}
}

func appendPolicies(resources int) map[string]filament.WritePolicy {
	policies := make(map[string]filament.WritePolicy, resources)
	for i := range resources {
		resource := "res-" + strconv.Itoa(i)
		policies[resource] = appendPolicy(resource)
	}
	return policies
}
