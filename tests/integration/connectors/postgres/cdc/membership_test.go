//go:build integration

package cdc_test

import (
	"context"
	"fmt"
	"slices"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

// A live stream admits a pending table by reading its baseline pinned at one
// exported point, flooring that table's older changes, and reporting it as
// activated. A removed table leaves the publication. A resynced table is read
// in full again. Tables already streaming keep their bookmark throughout.
func TestPostgresContinuousMembership(t *testing.T) {
	pg := testcontainers.SharedPostgresCDC(t)
	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()
	exec := func(sql string) {
		t.Helper()
		if _, err := pg.Pool().Exec(ctx, sql); err != nil {
			t.Fatal(err)
		}
	}
	exec(`CREATE TABLE member_a (id bigint PRIMARY KEY, name text NOT NULL);
 CREATE TABLE member_b (id bigint PRIMARY KEY, name text NOT NULL);
 ALTER TABLE member_a REPLICA IDENTITY FULL; ALTER TABLE member_b REPLICA IDENTITY FULL;
 INSERT INTO member_a VALUES (1, 'a'); INSERT INTO member_b VALUES (1, 'b');`)
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "replication": "cdc", "max_conns": 1})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(context.Background()) }()
	plan, err := src.PlanReplicationStream(filament.ReplicationStreamPlanningRequest{ReplicationStreamID: "membership-test"})
	if err != nil {
		t.Fatal(err)
	}
	opts := filament.StreamOpenOpts{
		Resources:  []string{"member_a"},
		Membership: map[string]filament.ReplicationStreamResourceStatus{"member_a": filament.ReplicationStreamResourceActive},
		Attempt:    filament.AttemptRef{RunID: "run", ExecutionID: "execution", StreamID: "membership-test", Generation: 1, Token: 1}, MembershipRevision: 1,
		CheckAuthority: func(context.Context) error { return nil },
	}
	var out *continuousCollector
	open := func() filament.StreamSession {
		t.Helper()
		out = &continuousCollector{}
		t.Cleanup(out.Release)
		session, err := src.OpenStream(ctx, opts)
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = session.Close(context.Background()) })
		return session
	}
	read := func(session filament.StreamSession) filament.Coverage {
		t.Helper()
		out.controls = nil
		coverage, err := session.Read(ctx, out, filament.Boundary{MaxRecords: 100, MaxWait: time.Second, MaxAge: time.Second})
		if err != nil {
			t.Fatal(err)
		}
		if err := session.Acknowledge(ctx, coverage); err != nil {
			t.Fatal(err)
		}
		return coverage
	}
	ops := map[filament.Operation]string{filament.OpInsert: "insert", filament.OpUpdate: "update", filament.OpDelete: "delete"}
	rows := func(resource string) []string {
		var got []string
		for _, record := range testutil.DataRecords(out.Records) {
			if record.Resource == resource {
				got = append(got, fmt.Sprintf("%s:%s:%v", ops[record.Op], record.ID, record.LSN != ""))
			}
		}
		return got
	}
	published := func() []string {
		rows, err := pg.Pool().Query(ctx, `SELECT tablename FROM pg_publication_tables WHERE pubname = $1 ORDER BY tablename`, plan.ConsumerName)
		if err != nil {
			t.Fatal(err)
		}
		defer rows.Close()
		var tables []string
		for rows.Next() {
			var table string
			if err := rows.Scan(&table); err != nil {
				t.Fatal(err)
			}
			tables = append(tables, table)
		}
		return tables
	}

	// First bootstrap: the stream's own snapshot of its only member.
	session := open()
	first := read(session)
	if got := rows("member_a"); !slices.Equal(got, []string{"insert:1:false"}) || len(first.Activated) != 0 {
		t.Fatalf("first baseline = %v activated=%v", got, first.Activated)
	}
	if err := session.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// Changes land on both tables before the second table joins. The joining
	// table's change is inside its baseline and must not replay from WAL.
	exec(`INSERT INTO member_a VALUES (2, 'a'); INSERT INTO member_b VALUES (2, 'b');`)
	opts.Resources = []string{"member_a", "member_b"}
	opts.Membership["member_b"] = filament.ReplicationStreamResourcePending
	opts.CommittedPositions, opts.MembershipRevision = first.Positions, 2
	opts.Attempt.Token++
	session = open()
	joined := read(session)
	if !slices.Equal(joined.Activated, []string{"member_b"}) {
		t.Fatalf("activated = %v, want member_b", joined.Activated)
	}
	if got := rows("member_a"); !slices.Equal(got, []string{"insert:2:true"}) {
		t.Fatalf("streaming member kept its bookmark: %v", got)
	}
	if got := rows("member_b"); !slices.Equal(got, []string{"insert:1:false", "insert:2:false"}) {
		t.Fatalf("joining member baseline = %v, want both rows from the snapshot only", got)
	}
	activated := slices.IndexFunc(out.controls, func(c filament.Control) bool { return c.Kind == filament.MemberActivated })
	if activated < 0 || out.controls[activated].Resource != "member_b" || out.controls[activated].MembershipRevision != 2 {
		t.Fatalf("member activation control missing: %#v", out.controls)
	}
	if got := published(); !slices.Equal(got, []string{"member_a", "member_b"}) {
		t.Fatalf("publication = %v", got)
	}
	exec(`INSERT INTO member_b VALUES (3, 'b');`)
	streamed := read(session)
	if got := rows("member_b"); !slices.Equal(got, []string{"insert:1:false", "insert:2:false", "insert:3:true"}) || len(streamed.Activated) != 0 {
		t.Fatalf("joined member must stream past its baseline: %v activated=%v", got, streamed.Activated)
	}
	if err := session.Close(ctx); err != nil {
		t.Fatal(err)
	}

	// Removing the first table drops it from the publication. Resyncing the
	// second reads it in full again while the slot keeps its position.
	opts.Resources = []string{"member_b"}
	opts.Membership = map[string]filament.ReplicationStreamResourceStatus{"member_b": filament.ReplicationStreamResourcePending}
	opts.CommittedPositions, opts.MembershipRevision = streamed.Positions, 3
	opts.Attempt.Token++
	session = open()
	resynced := read(session)
	if got := published(); !slices.Equal(got, []string{"member_b"}) {
		t.Fatalf("publication after removal = %v", got)
	}
	if got := rows("member_b"); !slices.Equal(got, []string{"insert:1:false", "insert:2:false", "insert:3:false"}) || !slices.Equal(resynced.Activated, []string{"member_b"}) {
		t.Fatalf("resync baseline = %v activated=%v", got, resynced.Activated)
	}
	if got := rows("member_a"); len(got) != 0 {
		t.Fatalf("removed member still decoded: %v", got)
	}
	if err := session.Close(ctx); err != nil {
		t.Fatal(err)
	}
}
