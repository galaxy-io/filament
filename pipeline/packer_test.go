package pipeline

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/integrations/alchemy"
	"github.com/galaxy-io/filament/transform"
)

var (
	upperRef  = transform.FunctionRef{ID: "fn_upper", Version: 1}
	upperStep = &transform.Definition{Version: 1, Resources: map[string]transform.Resource{
		"users": {Steps: []transform.Step{{Alchemy: &transform.AlchemyStep{Function: upperRef.ID, Version: upperRef.Version}}}},
	}}
	nField = arrow.NewSchema([]arrow.Field{{Name: "id", Type: arrow.PrimitiveTypes.Int64}, {Name: "n", Type: arrow.BinaryTypes.String, Nullable: true}}, nil)
)

// functionsFunc adapts a function over the input record to Functions.
type functionsFunc func(in arrow.RecordBatch) (alchemy.Result, error)

func (f functionsFunc) Invoke(_ context.Context, _ alchemy.Request, in arrow.RecordBatch) (alchemy.Result, error) {
	return f(in)
}

// xNames echoes the input's ids with every n set to X.
func xNames(in arrow.RecordBatch) alchemy.Result {
	b := array.NewStringBuilder(memory.DefaultAllocator)
	defer b.Release()
	for range in.NumRows() {
		b.Append("X")
	}
	col := b.NewArray()
	defer col.Release()
	return alchemy.Result{Record: array.NewRecordBatch(nField, []arrow.Array{in.Column(0), col}, in.NumRows())}
}

// idSink records the ids and n values of every batch it writes, in order.
type idSink struct {
	fakeSink
	imu   sync.Mutex
	seqs  []uint64
	ids   [][]int64
	upper bool
}

func (s *idSink) Apply(ctx context.Context, b *arrowbatch.Batch, opts filament.ApplyOptions) (filament.WriteReceipt, error) {
	rows := b.Rows()
	ids := rows.Column(0).(*array.Int64).Int64Values()
	names := rows.Column(1).(*array.String)
	s.imu.Lock()
	s.seqs = append(s.seqs, b.Seq)
	s.ids = append(s.ids, append([]int64(nil), ids...))
	s.upper = names.Len() > 0 && names.Value(0) == "X"
	s.imu.Unlock()
	return s.fakeSink.Apply(ctx, b, opts)
}

func (s *idSink) written() ([]uint64, [][]int64) {
	s.imu.Lock()
	defer s.imu.Unlock()
	return append([]uint64(nil), s.seqs...), append([][]int64(nil), s.ids...)
}

func userRowsN(n int) []row {
	rows := make([]row, n)
	for i := range rows {
		rows[i] = rec("users", int64(i+1))
	}
	return rows
}

func runPacked(t *testing.T, sink filament.Sink, fns Functions, cfg Config, rows []row) error {
	t.Helper()
	cfg.Tenant, cfg.Run, cfg.Sink = "t1", "r1", sink
	cfg.WritePolicies = defaultWritePolicies(rows)
	cfg.Transform = upperStep
	if cfg.Contracts == nil {
		cfg.Contracts = transform.ContractMap{upperRef: {Input: nField, Output: nField}}
	}
	cfg.Functions = fns
	if cfg.FlushInterval == 0 {
		cfg.FlushInterval = time.Hour
	}
	if cfg.Options.BatchMaxRows == 0 {
		cfg.Options.BatchMaxRows = 2
	}
	p := New(cfg)
	p.Start(context.Background())
	err := push(p.Records(), rows)
	p.CloseIngest(err)
	return p.Wait()
}

func TestPackerPacksSmallBatchesIntoOneCall(t *testing.T) {
	var calls []int64
	sink := &idSink{}
	fns := functionsFunc(func(in arrow.RecordBatch) (alchemy.Result, error) {
		calls = append(calls, in.NumRows())
		return xNames(in), nil
	})
	if err := runPacked(t, sink, fns, Config{}, userRowsN(10)); err != nil {
		t.Fatal(err)
	}
	if len(calls) != 1 || calls[0] != 10 {
		t.Fatalf("calls = %v, want one call of 10 rows", calls)
	}
	seqs, ids := sink.written()
	if len(seqs) != 5 || !sink.upper {
		t.Fatalf("wrote %d batches (upper=%v), want the 5 original batches transformed", len(seqs), sink.upper)
	}
	for i := 1; i < len(seqs); i++ {
		if seqs[i] <= seqs[i-1] || ids[i][0] <= ids[i-1][0] {
			t.Fatalf("batches out of order: seqs %v ids %v", seqs, ids)
		}
	}
}

func TestPackerFailureWritesNothingAtOrAfterIt(t *testing.T) {
	boom := errors.New("function failed after retries")
	sink := &idSink{}
	fns := functionsFunc(func(in arrow.RecordBatch) (alchemy.Result, error) {
		for _, id := range in.Column(0).(*array.Int64).Int64Values() {
			if id == 5 {
				return alchemy.Result{}, boom
			}
		}
		time.Sleep(5 * time.Millisecond)
		return xNames(in), nil
	})
	cfg := Config{FunctionBatch: FunctionBatch{MaxBytes: 1, MaxInFlight: 4}}
	err := runPacked(t, sink, fns, cfg, userRowsN(20))
	if !errors.Is(err, boom) {
		t.Fatalf("Wait() = %v, want %v", err, boom)
	}
	_, ids := sink.written()
	for _, batch := range ids {
		if batch[0] >= 5 {
			t.Fatalf("wrote batch starting at id %d at or after the failed pack: %v", batch[0], ids)
		}
	}
}

func TestPackerReleasesEverything(t *testing.T) {
	mem := memory.NewCheckedAllocator(memory.NewGoAllocator())
	defer mem.AssertSize(t, 0)
	fns := functionsFunc(func(in arrow.RecordBatch) (alchemy.Result, error) { return xNames(in), nil })
	if err := runPacked(t, &idSink{}, fns, Config{Allocator: mem}, userRowsN(9)); err != nil {
		t.Fatal(err)
	}
}
