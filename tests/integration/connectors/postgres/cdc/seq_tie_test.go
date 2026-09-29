//go:build integration

package cdc_test

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	"github.com/galaxy-io/filament/tests/testcontainers"
)

func TestReproSeqTie(t *testing.T) {
	pg := testcontainers.SharedPostgresCDC(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	const resource = "repro_tie"
	if _, err := pg.Pool().Exec(ctx, `CREATE TABLE repro_tie (id bigint PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "slot_name": "filament_seq_tie", "publication": "filament_seq_tie",
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(context.Background()) }()
	initial := &testutil.CollectSink{}
	defer initial.Release()
	if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{resource}}); err != nil {
		t.Fatal(err)
	}
	start := testutil.StreamCheckpoint(t, initial.Records, resource)
	if _, err := pg.Pool().Exec(ctx, `INSERT INTO repro_tie VALUES (1), (2), (3)`); err != nil {
		t.Fatal(err)
	}
	changes := &testutil.CollectSink{}
	defer changes.Release()
	if err := src.ExtractChanges(ctx, changes, filament.ChangeExtractOpts{
		Resources: []string{resource}, Checkpoints: map[string]filament.Checkpoint{resource: start},
	}); err != nil {
		t.Fatal(err)
	}
	if rows := testutil.DataRecords(changes.Records); len(rows) != 3 {
		t.Fatalf("got %d rows, want 3", len(rows))
	}
	mark := testutil.StreamCheckpoint(t, changes.Records, resource)
	// A drain can be folded before an earlier batch completes. Fold all rows
	// after the drain to exercise the emitter's out-of-order completion path.
	var folded filament.Checkpoint = mark
	for _, rec := range changes.Records {
		if !rec.Drained {
			folded = checkpoint.MergeStream(folded, checkpoint.NewStreamDelta(resource, rec.LSN, rec.Seq))
		}
	}
	wantLSN, _, _ := checkpoint.ParseStream(mark)
	if gotLSN, _, _ := checkpoint.ParseStream(folded); gotLSN != wantLSN {
		t.Fatalf("folded cursor regressed from %s to %s", wantLSN, gotLSN)
	}
	idle := &testutil.CollectSink{}
	defer idle.Release()
	if err := src.ExtractChanges(ctx, idle, filament.ChangeExtractOpts{
		Resources: []string{resource}, Checkpoints: map[string]filament.Checkpoint{resource: folded},
	}); err != nil {
		t.Fatal(err)
	}
	if rows := testutil.DataRecords(idle.Records); len(rows) != 0 {
		t.Fatalf("resuming replayed previous cycle's rows: %#v", rows)
	}
}
