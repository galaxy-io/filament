//go:build integration

package cdc_test

import (
	"context"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

type continuousCollector struct {
	duplicateRejectingSink
	controls []filament.Control
}

func (s *continuousCollector) Control(_ context.Context, c filament.Control) error {
	s.controls = append(s.controls, c)
	return nil
}

func TestPostgresContinuousTransactionsAndResume(t *testing.T) {
	pg := testcontainers.SharedPostgresCDC(t)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	exec := func(sql string) {
		t.Helper()
		if _, err := pg.Pool().Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE continuous_rows (id bigint PRIMARY KEY, name text NOT NULL);
 ALTER TABLE continuous_rows REPLICA IDENTITY FULL;
 INSERT INTO continuous_rows VALUES (1, 'baseline');`)
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "replication": "cdc", "snapshot_mode": "none", "publication": "continuous_rows_pub",
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(context.Background()) }()
	opts := filament.StreamOpenOpts{
		Resources:      []string{"continuous_rows"},
		Attempt:        filament.AttemptRef{RunID: "run", ExecutionID: "execution", StreamID: "continuous-test", Generation: 1, Token: 1},
		CheckAuthority: func(context.Context) error { return nil },
	}
	var out *continuousCollector
	open := func() filament.StreamSession {
		out = &continuousCollector{}
		t.Cleanup(out.Release)
		t.Helper()
		session, err := src.OpenStream(ctx, opts)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close(context.Background()) })
		return session
	}
	read := func(session filament.StreamSession, want int) filament.Coverage {
		t.Helper()
		before := len(out.Records)
		out.controls = nil
		coverage, err := session.Read(ctx, out, filament.Boundary{MaxRecords: 1, MaxWait: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if records := testutil.DataRecords(out.Records[before:]); len(records) != want {
			t.Fatalf("got %d rows, want %d: %#v", len(records), want, records)
		}
		if want > 0 && (len(out.controls) != 3 || out.controls[0].Kind != filament.TxnBegin || out.controls[1].Kind != filament.TxnEnd) {
			t.Fatalf("incomplete transaction controls: %#v", out.controls)
		}
		return coverage
	}
	session := open()
	// Existing rows are not snapshotted. A two-row transaction exceeds the soft
	// one-record target but must stay in one certified boundary.
	exec(`INSERT INTO continuous_rows VALUES (2, 'first'), (3, 'first');`)
	first := read(session, 2)
	if err := session.Acknowledge(ctx, first); err != nil {
		t.Fatal(err)
	}
	exec(`UPDATE continuous_rows SET name='second' WHERE id=2;`)
	second := read(session, 1)
	// Leave this boundary unacknowledged, as if its destination commit failed.
	if err := session.Close(ctx); err != nil {
		t.Fatal(err)
	}
	opts.CommittedPositions = first.Positions
	opts.Attempt.Token++
	session = open()
	replay := read(session, 1)
	for domain, position := range second.Positions {
		if string(replay.Positions[domain].Value) != string(position.Value) {
			t.Fatal("replay position changed")
		}
	}
	if err := session.Acknowledge(ctx, replay); err != nil {
		t.Fatal(err)
	}
	if coverage := read(session, 0); len(coverage.Positions) != 0 {
		t.Fatal("idle read advanced coverage")
	}
	if err := session.Close(ctx); err != nil {
		t.Fatal(err)
	}
	// A certified stream may never silently create a replacement slot.
	plan, err := src.PlanReplicationStream(filament.ReplicationStreamPlanningRequest{ReplicationStreamID: opts.Attempt.StreamID})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Pool().Exec(ctx, `SELECT pg_drop_replication_slot($1)`, plan.ConsumerName); err != nil {
		t.Fatal(err)
	}
	opts.CommittedPositions = replay.Positions
	if unexpected, err := src.OpenStream(ctx, opts); err == nil {
		_ = unexpected.Close(ctx)
		t.Fatal("missing certified slot was recreated")
	}
}
