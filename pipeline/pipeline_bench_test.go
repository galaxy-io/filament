package pipeline

import (
	"context"
	"fmt"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
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

func (nullSink) Write(_ context.Context, b filament.Batch) (filament.WriteReceipt, error) {
	crc, nbytes := filament.CRC32C(b.Records)
	return filament.WriteReceipt{WriteCRC: crc, Bytes: nbytes, Rows: len(b.Records)}, nil
}

func (nullSink) Apply(ctx context.Context, b filament.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	if err := opts.Policy.ValidateRecords(b.Resource, b.Records); err != nil {
		return filament.WriteReceipt{}, err
	}
	return nullSink{}.Write(ctx, b)
}

// benchRecords builds n ~80-byte JSON records with unique ids. Built once in
// setup and reused across iterations, so the allocs reported are the pipeline's,
// not the fixture's.
func benchRecords(n int) []filament.Record {
	out := make([]filament.Record, n)
	for i := range out {
		id := strconv.Itoa(i)
		data := []byte(`{"i":` + id + `,"name":"row","payload":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}`)
		out[i] = filament.NewRecord("bench", id, data)
	}
	return out
}

// BenchmarkNullSinkPipeline is the top rung of the subtraction ladder: records
// pushed through the full batcher→writer→integrity loop into a zero-I/O sink, no
// source read, bus, or storage. allocs/op ÷ rows is the pipeline's own floor —
// whatever the engine bench costs above this is source read + bus + tracker.
//
// Note the integrity path canonical-encodes each record twice here (batcher
// ReadCRC + nullSink WriteCRC), so this rung is also the headline witness for
// Round 2's "canonical once / append encoder" change.
func BenchmarkNullSinkPipeline(b *testing.B) {
	const rows = 20_000
	recs := benchRecords(rows)
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
		in := p.Records()
		for i := range recs {
			if err := in.Push(recs[i]); err != nil {
				b.Fatalf("push: %v", err)
			}
		}
		p.CloseIngest()
		if err := p.Wait(); err != nil {
			b.Fatalf("wait: %v", err)
		}
		processed += rows
	}
	if s := b.Elapsed().Seconds(); s > 0 {
		b.ReportMetric(float64(processed)/s, "records/sec")
	}
}

func appendPolicy(resource string) filament.WritePolicy {
	policy := filament.WritePolicyForIngestion(filament.IngestionAppend)
	policy.Resource = resource
	return policy
}

// BenchmarkNullSinkPipelineParallel is the Docker-free ceiling with parallelism:
// records span several resources (so the batcher sharding engages) and are pushed
// from a pool of goroutines (mimicking a parallel source). Sweeping parallelism
// shows the pipeline itself scaling once batching and the read CRC are no longer
// single-goroutine — isolating that from the shared-database contention the macro
// engine benches hit.
func BenchmarkNullSinkPipelineParallel(b *testing.B) {
	const rows = 40_000
	const resources = 8
	recs := make([]filament.Record, rows)
	for i := range recs {
		id := strconv.Itoa(i)
		data := []byte(`{"i":` + id + `,"name":"row","payload":"xxxxxxxxxxxxxxxxxxxxxxxxxxxxxxxx"}`)
		recs[i] = filament.NewRecord("res-"+strconv.Itoa(i%resources), id, data)
	}
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
				for w := range parallelism {
					wg.Add(1)
					go func(w int) {
						defer wg.Done()
						for i := w; i < len(recs); i += parallelism {
							if err := in.Push(recs[i]); err != nil {
								return // pipeline failed; Wait surfaces it
							}
						}
					}(w)
				}
				wg.Wait()
				pl.CloseIngest()
				if err := pl.Wait(); err != nil {
					b.Fatalf("wait: %v", err)
				}
				processed += rows
			}
			if s := b.Elapsed().Seconds(); s > 0 {
				b.ReportMetric(float64(processed)/s, "records/sec")
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
