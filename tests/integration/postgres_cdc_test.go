//go:build integration

package integration

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	pgsink "github.com/galaxy-io/filament/connectors/postgres/sink"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

// duplicateRejectingSink matches the pipeline inlet's one-builder-per-part
// contract, which catches bootstrap code that bypasses the CDC writer cache.
type duplicateRejectingSink struct {
	collectSink
	buildersMu sync.Mutex
	builders   map[struct {
		resource string
		part     int
	}]struct{}
}

func (s *duplicateRejectingSink) Builder(resource string, part int, schema filament.RecordSchema) (filament.RowWriter, error) {
	s.buildersMu.Lock()
	defer s.buildersMu.Unlock()
	key := struct {
		resource string
		part     int
	}{resource: resource, part: part}
	if _, exists := s.builders[key]; exists {
		return nil, fmt.Errorf("builder already open for %q part %d", resource, part)
	}
	if s.builders == nil {
		s.builders = make(map[struct {
			resource string
			part     int
		}]struct{})
	}
	s.builders[key] = struct{}{}
	return s.collectSink.Builder(resource, part, schema)
}

// snapshotBarrierSink blocks the source at its first snapshot row (a row with no
// stream position) until released, so the test can commit changes while the
// bootstrap snapshot is still open.
type snapshotBarrierSink struct {
	collectSink
	once    sync.Once
	reached chan struct{}
	release chan struct{}
}

func newSnapshotBarrierSink() *snapshotBarrierSink {
	s := &snapshotBarrierSink{reached: make(chan struct{}), release: make(chan struct{})}
	s.wrap = func(w filament.RowWriter) filament.RowWriter { return &barrierWriter{RowWriter: w, sink: s} }
	return s
}

type barrierWriter struct {
	filament.RowWriter
	sink *snapshotBarrierSink
}

func (w *barrierWriter) EndRow(meta filament.RowMeta) error {
	if meta.LSN == "" {
		w.sink.once.Do(func() {
			close(w.sink.reached)
			<-w.sink.release
		})
	}
	return w.RowWriter.EndRow(meta)
}

func TestPostgresCDCBootstrapSnapshotThenWAL(t *testing.T) {
	pg := testcontainers.SharedPostgresCDC(t)
	// The timeout budgets the pipeline, not the container boot above it.
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if _, err := pg.Pool().Exec(ctx, `
		CREATE TABLE cdc_handoff (id bigint PRIMARY KEY, name text NOT NULL);
		ALTER TABLE cdc_handoff REPLICA IDENTITY FULL;
		INSERT INTO cdc_handoff VALUES (1, 'Before');
	`); err != nil {
		t.Fatal(err)
	}

	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "slot_name": "filament_test_handoff", "publication": "filament_test_handoff",
		"max_conns": 1,
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(context.Background()) }()

	sink := newSnapshotBarrierSink()
	errCh := make(chan error, 1)
	go func() {
		errCh <- src.ExtractChanges(ctx, sink, filament.ChangeExtractOpts{Resources: []string{"cdc_handoff"}})
	}()

	select {
	case <-sink.reached:
	case err := <-errCh:
		t.Fatalf("CDC bootstrap ended before snapshot barrier: %v", err)
	case <-ctx.Done():
		t.Fatal(ctx.Err())
	}

	// Commit while the exported snapshot scan is paused. This transaction is not
	// visible in the snapshot, so it must arrive from WAL after the baseline row.
	if _, err := pg.Pool().Exec(ctx, `
		UPDATE cdc_handoff SET name = 'During' WHERE id = 1;
		INSERT INTO cdc_handoff VALUES (2, 'During');
	`); err != nil {
		close(sink.release)
		t.Fatal(err)
	}
	close(sink.release)
	if err := <-errCh; err != nil {
		t.Fatal(err)
	}

	records := dataRecs(sink.recs)
	if len(records) != 3 {
		t.Fatalf("snapshot/WAL records = %#v, want baseline insert, WAL update, WAL insert", records)
	}
	if records[0].ID != "1" || records[0].Op != filament.OpInsert || records[0].LSN != "" {
		t.Fatalf("first record = %#v, want snapshot insert for row 1", records[0])
	}
	if records[1].ID != "1" || records[1].Op != filament.OpUpdate || records[1].LSN == "" {
		t.Fatalf("second record = %#v, want WAL update for row 1", records[1])
	}
	if records[2].ID != "2" || records[2].Op != filament.OpInsert || records[2].LSN == "" {
		t.Fatalf("third record = %#v, want WAL insert for row 2", records[2])
	}
	_ = streamCheckpointFromRecords(t, sink.recs, "cdc_handoff")
}

func TestPostgresCDCCatchupAndResume(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	if _, err := pg.Pool().Exec(ctx, `
		CREATE TABLE cdc_users (id bigint PRIMARY KEY, name text NOT NULL);
		ALTER TABLE cdc_users REPLICA IDENTITY FULL;
		INSERT INTO cdc_users VALUES (10, 'Existing');
	`); err != nil {
		t.Fatal(err)
	}

	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "slot_name": "filament_test_cdc", "publication": "filament_test_cdc",
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()

	initial := &duplicateRejectingSink{}
	if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{"cdc_users"}}); err != nil {
		t.Fatal(err)
	}
	initialData := dataRecs(initial.recs)
	if len(initialData) != 1 || initialData[0].ID != "10" || initialData[0].Op != filament.OpInsert {
		t.Fatalf("initial CDC snapshot = %#v, want existing row 10", initialData)
	}
	first := streamCheckpointFromRecords(t, initial.recs, "cdc_users")

	// Simulate an interrupted bootstrap whose slot survived but whose checkpoint
	// did not. The retry must take another full snapshot before resuming the slot.
	retry := &collectSink{}
	if err := src.ExtractChanges(ctx, retry, filament.ChangeExtractOpts{Resources: []string{"cdc_users"}}); err != nil {
		t.Fatal(err)
	}
	retryData := dataRecs(retry.recs)
	if len(retryData) != 1 || retryData[0].ID != "10" {
		t.Fatalf("retried CDC snapshot = %#v, want existing row 10", retryData)
	}

	if _, err := pg.Pool().Exec(ctx, `
		INSERT INTO cdc_users VALUES (1, 'Ada');
		UPDATE cdc_users SET id=2, name='Grace' WHERE id=1;
		DELETE FROM cdc_users WHERE id=2;
	`); err != nil {
		t.Fatal(err)
	}
	changes := &collectSink{}
	if err := src.ExtractChanges(ctx, changes, filament.ChangeExtractOpts{
		Resources: []string{"cdc_users"}, Checkpoints: map[string]filament.Checkpoint{"cdc_users": first},
	}); err != nil {
		t.Fatal(err)
	}
	var ops []filament.Operation
	for _, rec := range changes.recs {
		if !rec.Drained {
			ops = append(ops, rec.Op)
		}
	}
	want := []filament.Operation{filament.OpInsert, filament.OpDelete, filament.OpInsert, filament.OpDelete}
	if len(ops) != len(want) {
		t.Fatalf("CDC ops = %v, want %v; records = %#v", ops, want, changes.recs)
	}
	for i := range want {
		if ops[i] != want[i] {
			t.Fatalf("CDC ops = %v, want %v", ops, want)
		}
	}

	// Exercise the PostgreSQL sink's ordered merge path with the decoded stream.
	dst := pgsink.New()
	if err := dst.Open(ctx, filament.RunSpec{
		Run: "cdc-sink", IngestionTypes: map[string]filament.IngestionType{"": filament.IngestionCDCMerge},
		Sink: filament.Ref{Config: map[string]any{"dsn": pg.DSN(), "schema": "cdc_dst"}},
	}); err != nil {
		t.Fatal(err)
	}
	schema, err := src.Schema(ctx, "cdc_users")
	if err != nil {
		t.Fatal(err)
	}
	if err := dst.EnsureSchema(ctx, "cdc_users", schema); err != nil {
		t.Fatal(err)
	}
	policy := filament.WritePolicyForIngestion(filament.IngestionCDCMerge)
	policy.Resource = "cdc_users"
	policy.Keys = []string{"id"}
	if err := applyAll(ctx, dst, initial.batchesFor("cdc_users"), policy); err != nil {
		t.Fatal(err)
	}
	if err := applyAll(ctx, dst, changes.batchesFor("cdc_users"), policy); err != nil {
		t.Fatal(err)
	}
	if err := dst.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var destinationRows int
	if err := pg.Pool().QueryRow(ctx, `SELECT count(*) FROM cdc_dst.cdc_users`).Scan(&destinationRows); err != nil {
		t.Fatal(err)
	}
	if destinationRows != 1 {
		t.Fatalf("CDC destination rows = %d, want only the snapshotted row", destinationRows)
	}

	next := streamCheckpointFromRecords(t, changes.recs, "cdc_users")
	idle := &collectSink{}
	if err := src.ExtractChanges(ctx, idle, filament.ChangeExtractOpts{
		Resources: []string{"cdc_users"}, Checkpoints: map[string]filament.Checkpoint{"cdc_users": next},
	}); err != nil {
		t.Fatal(err)
	}
	for _, rec := range idle.recs {
		if !rec.Drained {
			t.Fatalf("idle scheduled catch-up replayed data: %#v", idle.recs)
		}
	}
}

func dataRecs(records []rec) []rec {
	out := make([]rec, 0, len(records))
	for _, r := range records {
		if !r.Drained {
			out = append(out, r)
		}
	}
	return out
}

func streamCheckpointFromRecords(t *testing.T, records []rec, resource string) filament.Checkpoint {
	t.Helper()
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Resource == resource && records[i].Drained && records[i].LSN != "" {
			return checkpoint.NewStreamDelta(resource, records[i].LSN, records[i].Seq)
		}
	}
	t.Fatalf("no stream marker for %q in %#v", resource, records)
	return nil
}
