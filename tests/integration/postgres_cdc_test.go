//go:build integration

package integration

import (
	"context"
	"testing"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	pgsink "github.com/galaxy-io/filament/connectors/postgres/sink"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

func TestPostgresCDCCatchupAndResume(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.Postgres(t,
		testcontainers.WithImage("postgres:16-alpine"),
		testcontainers.WithLogicalReplication(),
	)
	if _, err := pg.Pool().Exec(ctx, `
		CREATE TABLE cdc_users (id bigint PRIMARY KEY, name text NOT NULL);
		ALTER TABLE cdc_users REPLICA IDENTITY FULL;
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

	initial := &collectSink{}
	if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{"cdc_users"}}); err != nil {
		t.Fatal(err)
	}
	first := streamCheckpointFromRecords(t, initial.recs, "cdc_users")

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
		Run: "cdc-sink", IngestionType: filament.IngestionCDC,
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
	dataRecords := make([]filament.Record, 0, len(changes.recs))
	for _, rec := range changes.recs {
		if !rec.Drained {
			dataRecords = append(dataRecords, rec)
		}
	}
	policy := filament.WritePolicyForIngestion(filament.IngestionCDC)
	policy.Resource = "cdc_users"
	policy.Keys = []string{"id"}
	if _, err := dst.Apply(ctx, filament.Batch{Resource: "cdc_users", Records: dataRecords}, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if err := dst.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	var destinationRows int
	if err := pg.Pool().QueryRow(ctx, `SELECT count(*) FROM cdc_dst.cdc_users`).Scan(&destinationRows); err != nil {
		t.Fatal(err)
	}
	if destinationRows != 0 {
		t.Fatalf("CDC destination rows = %d, want 0", destinationRows)
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

func streamCheckpointFromRecords(t *testing.T, records []filament.Record, resource string) filament.Checkpoint {
	t.Helper()
	for i := len(records) - 1; i >= 0; i-- {
		if records[i].Resource == resource && records[i].Drained && records[i].Meta.LSN != "" {
			return checkpoint.NewStreamDelta(resource, records[i].Meta.LSN, records[i].Meta.Seq)
		}
	}
	t.Fatalf("no stream marker for %q in %#v", resource, records)
	return nil
}
