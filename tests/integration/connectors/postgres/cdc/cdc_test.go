//go:build integration

package cdc_test

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
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

// duplicateRejectingSink matches the pipeline inlet's one-builder-per-part
// contract, which catches bootstrap code that bypasses the CDC writer cache.
type duplicateRejectingSink struct {
	testutil.CollectSink
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
	return s.CollectSink.Builder(resource, part, schema)
}

// snapshotBarrierSink blocks the source at its first snapshot row (a row with no
// stream position) until released, so the test can commit changes while the
// bootstrap snapshot is still open.
type snapshotBarrierSink struct {
	testutil.CollectSink
	once    sync.Once
	reached chan struct{}
	release chan struct{}
}

func newSnapshotBarrierSink() *snapshotBarrierSink {
	s := &snapshotBarrierSink{reached: make(chan struct{}), release: make(chan struct{})}
	s.SetWriterWrapper(func(w filament.RowWriter) filament.RowWriter {
		return &barrierWriter{RowWriter: w, sink: s}
	})
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

	records := testutil.DataRecords(sink.Records)
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
	_ = testutil.StreamCheckpoint(t, sink.Records, "cdc_handoff")
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
	initialData := testutil.DataRecords(initial.Records)
	if len(initialData) != 1 || initialData[0].ID != "10" || initialData[0].Op != filament.OpInsert {
		t.Fatalf("initial CDC snapshot = %#v, want existing row 10", initialData)
	}
	first := testutil.StreamCheckpoint(t, initial.Records, "cdc_users")

	// Simulate an interrupted bootstrap whose slot survived but whose checkpoint
	// did not. The retry must take another full snapshot before resuming the slot.
	retry := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, retry, filament.ChangeExtractOpts{Resources: []string{"cdc_users"}}); err != nil {
		t.Fatal(err)
	}
	retryData := testutil.DataRecords(retry.Records)
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
	changes := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, changes, filament.ChangeExtractOpts{
		Resources: []string{"cdc_users"}, Checkpoints: map[string]filament.Checkpoint{"cdc_users": first},
	}); err != nil {
		t.Fatal(err)
	}
	var ops []filament.Operation
	for _, rec := range changes.Records {
		if !rec.Drained {
			ops = append(ops, rec.Op)
		}
	}
	want := []filament.Operation{filament.OpInsert, filament.OpDelete, filament.OpInsert, filament.OpDelete}
	if len(ops) != len(want) {
		t.Fatalf("CDC ops = %v, want %v; records = %#v", ops, want, changes.Records)
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
	if err := testutil.ApplyAll(ctx, dst, initial.BatchesFor("cdc_users"), policy); err != nil {
		t.Fatal(err)
	}
	if err := testutil.ApplyAll(ctx, dst, changes.BatchesFor("cdc_users"), policy); err != nil {
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

	next := testutil.StreamCheckpoint(t, changes.Records, "cdc_users")
	// The runner invokes this only after the sink commit and tracker promotion.
	// Advancing here verifies the source releases WAL for the final completed
	// cycle instead of waiting for another extraction to start.
	if err := src.AcknowledgeChanges(ctx, map[string]filament.Checkpoint{"cdc_users": next}); err != nil {
		t.Fatal(err)
	}
	nextLSN, _, ok := checkpoint.ParseStream(next)
	if !ok {
		t.Fatalf("next checkpoint is not a stream cursor: %#v", next)
	}
	var acknowledged bool
	if err := pg.Pool().QueryRow(ctx, `SELECT confirmed_flush_lsn >= $2::pg_lsn
		FROM pg_replication_slots WHERE slot_name=$1`, "filament_test_cdc", nextLSN).Scan(&acknowledged); err != nil {
		t.Fatal(err)
	}
	if !acknowledged {
		t.Fatalf("slot confirmed_flush_lsn is behind acknowledged checkpoint %s", nextLSN)
	}

	idle := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, idle, filament.ChangeExtractOpts{
		Resources: []string{"cdc_users"}, Checkpoints: map[string]filament.Checkpoint{"cdc_users": next},
	}); err != nil {
		t.Fatal(err)
	}
	for _, rec := range idle.Records {
		if !rec.Drained {
			t.Fatalf("idle scheduled catch-up replayed data: %#v", idle.Records)
		}
	}
}

func TestPostgresCDCSnapshotModeNone(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	if _, err := pg.Pool().Exec(ctx, `
		CREATE TABLE cdc_nosnap (id bigint PRIMARY KEY, name text NOT NULL);
		ALTER TABLE cdc_nosnap REPLICA IDENTITY FULL;
		INSERT INTO cdc_nosnap VALUES (1, 'Existing');
	`); err != nil {
		t.Fatal(err)
	}

	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "slot_name": "filament_test_nosnap", "publication": "filament_test_nosnap",
		"snapshot_mode": "none",
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()

	// First run: a new slot, no baseline, only a stream mark at its consistent point.
	first := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, first, filament.ChangeExtractOpts{Resources: []string{"cdc_nosnap"}}); err != nil {
		t.Fatal(err)
	}
	if got := testutil.DataRecords(first.Records); len(got) != 0 {
		t.Fatalf("snapshot_mode none emitted %d baseline rows, want 0", len(got))
	}
	cp := testutil.StreamCheckpoint(t, first.Records, "cdc_nosnap")

	// Second run: only changes after that point.
	if _, err := pg.Pool().Exec(ctx, `
		INSERT INTO cdc_nosnap VALUES (2, 'Ada');
		UPDATE cdc_nosnap SET name='Changed' WHERE id=1;
	`); err != nil {
		t.Fatal(err)
	}
	second := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, second, filament.ChangeExtractOpts{
		Resources: []string{"cdc_nosnap"}, Checkpoints: map[string]filament.Checkpoint{"cdc_nosnap": cp},
	}); err != nil {
		t.Fatal(err)
	}
	data := testutil.DataRecords(second.Records)
	if len(data) != 2 || data[0].ID != "2" || data[0].Op != filament.OpInsert || data[1].ID != "1" || data[1].Op != filament.OpUpdate {
		t.Fatalf("post-start records = %#v, want WAL insert 2 then WAL update 1", data)
	}
}
