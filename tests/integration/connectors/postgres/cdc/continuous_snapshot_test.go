//go:build integration

package cdc_test

import (
	"context"
	"fmt"
	"net/url"
	"reflect"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/pipeline"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

type snapshotDestination struct {
	filament.Sink
	rows    map[string]string
	written int
}

func (*snapshotDestination) Spec() filament.SinkSpec { return filament.SinkSpec{} }
func (s *snapshotDestination) Apply(_ context.Context, b *arrowbatch.Batch, _ filament.ApplyOptions) (filament.WriteReceipt, error) {
	rows, ops := b.Rows(), b.Operations()
	for i := range int(rows.NumRows()) {
		id := rows.Column(0).ValueStr(i)
		if ops.Len() > 0 && ops.At(i) == filament.OpDelete {
			delete(s.rows, id)
		} else {
			s.rows[id] = rows.Column(1).ValueStr(i)
		}
		s.written++
	}
	return filament.WriteReceipt{URI: "test://snapshot", Rows: b.NumRows(), Bytes: b.Bytes(), WriteCRC: b.IntegrityCRC()}, nil
}

func TestPostgresContinuousSnapshotHandoffAndRecovery(t *testing.T) {
	for _, certified := range []bool{true, false} {
		t.Run(fmt.Sprintf("certified=%v", certified), func(t *testing.T) {
			pg := testcontainers.SharedPostgresCDC(t)
			ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
			defer cancel()
			name := fmt.Sprintf("snapshot_%v", certified)
			exec := func(sql string) {
				t.Helper()
				if _, err := pg.Pool().Exec(ctx, sql); err != nil {
					t.Fatal(err)
				}
			}
			exec("CREATE TABLE " + name + " (id bigint PRIMARY KEY, value text); ALTER TABLE " + name + " REPLICA IDENTITY FULL; INSERT INTO " + name + " VALUES (1,'old'), (2,'old')")
			dsn, err := url.Parse(pg.DSN())
			if err != nil {
				t.Fatal(err)
			}
			query := dsn.Query()
			query.Set("options", "-c wal_sender_timeout=1000")
			dsn.RawQuery = query.Encode()
			src := pgsource.New()
			// Omit snapshot_mode to exercise the default. A single pool connection
			// must work while the imported snapshot owns that connection.
			if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": dsn.String(), "replication": "cdc", "publication": name + "_pub", "max_conns": 1})); err != nil {
				t.Fatal(err)
			}
			defer func() { _ = src.Teardown(context.Background()) }()
			opts := filament.StreamOpenOpts{Resources: []string{name}, Attempt: filament.AttemptRef{RunID: "run", ExecutionID: "exec", StreamID: name, Generation: 1, Token: 1}, CheckAuthority: func(context.Context) error { return nil }}
			dst := &snapshotDestination{rows: map[string]string{}}
			var session filament.StreamSession
			var p *pipeline.Pipeline
			epoch := int64(0)
			open := func() {
				t.Helper()
				var err error
				session, err = src.OpenStream(ctx, opts)
				if err != nil {
					t.Fatal(err)
				}
				p, err = pipeline.NewStream(pipeline.Config{Sink: dst, WritePolicies: map[string]filament.WritePolicy{"": filament.WritePolicyForIngestion(filament.IngestionCDCMerge)}}, filament.OrderingGlobalStrict)
				if err != nil {
					t.Fatal(err)
				}
				p.Start(ctx)
				epoch = 0
			}
			closeSession := func() {
				t.Helper()
				if err := session.Close(ctx); err != nil {
					t.Fatal(err)
				}
				p.CloseIngest(nil)
				if err := p.Wait(); err != nil {
					t.Fatal(err)
				}
			}
			read := func() filament.Coverage {
				t.Helper()
				epoch++
				if err := p.BindEpoch(filament.EpochRef{Attempt: opts.Attempt, Epoch: epoch, MembershipRevision: 1}); err != nil {
					t.Fatal(err)
				}
				c, err := session.Read(ctx, p.Records().(filament.StreamRecordSink), filament.Boundary{MaxRecords: 1, MaxWait: 100 * time.Millisecond, MaxAge: 100 * time.Millisecond})
				if err != nil {
					t.Fatal(err)
				}
				if len(c.Positions) != 1 {
					t.Fatal("missing snapshot/WAL coverage")
				}
				if _, err := p.SealEpoch(); err != nil {
					t.Fatal(err)
				}
				return c
			}
			ack := func(c filament.Coverage) {
				t.Helper()
				if err := session.Acknowledge(ctx, c); err != nil {
					t.Fatal(err)
				}
			}
			open()
			// Changes after OpenStream must be excluded from the exported snapshot,
			// retained in WAL, and then emitted through the same pipeline builders.
			exec("DELETE FROM " + name + " WHERE id=1; UPDATE " + name + " SET value='new' WHERE id=2; INSERT INTO " + name + " VALUES (3,'new')")
			baseline := read()
			if dst.written != 2 || !reflect.DeepEqual(dst.rows, map[string]string{"1": "old", "2": "old"}) {
				t.Fatalf("snapshot=%v rows=%d", dst.rows, dst.written)
			}
			if certified {
				ack(baseline)
				// The statements share one transaction and one boundary.
				baseline = read()
				ack(baseline)
				opts.CommittedPositions = baseline.Positions
			}
			closeSession()
			opts.Attempt.Token++
			open()
			if !certified {
				// Re-snapshot after an uncertified write and replay retained deletes,
				// updates and inserts before returning one complete bootstrap boundary.
				baseline = read()
				ack(baseline)
			}
			if !reflect.DeepEqual(dst.rows, map[string]string{"2": "new", "3": "new"}) {
				t.Fatalf("recovered state=%v", dst.rows)
			}
			before := dst.written
			exec("INSERT INTO " + name + " VALUES (4,'after')")
			baseline = read()
			ack(baseline)
			if dst.written != before+1 || dst.rows["4"] != "after" {
				t.Fatalf("snapshot repeated or WAL lost: rows=%v writes=%d", dst.rows, dst.written-before)
			}
			closeSession()
		})
	}
}

func TestPostgresContinuousEmptySnapshotAndCloseBeforeRead(t *testing.T) {
	pg := testcontainers.SharedPostgresCDC(t)
	ctx, cancel := context.WithTimeout(t.Context(), 30*time.Second)
	defer cancel()
	if _, err := pg.Pool().Exec(ctx, "CREATE TABLE snapshot_empty (id bigint PRIMARY KEY)"); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "replication": "cdc", "publication": "snapshot_empty_pub", "max_conns": 1})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(context.Background()) }()
	opts := filament.StreamOpenOpts{Resources: []string{"snapshot_empty"}, Attempt: filament.AttemptRef{RunID: "run", ExecutionID: "exec", StreamID: "snapshotempty", Generation: 1, Token: 1}, CheckAuthority: func(context.Context) error { return nil }}
	session, err := src.OpenStream(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	// Closing without reading must release the snapshot's only pool connection.
	if err := session.Close(ctx); err != nil {
		t.Fatal(err)
	}
	session, err = src.OpenStream(ctx, opts)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = session.Close(context.Background()) }()
	out := &continuousCollector{}
	defer out.Release()
	c, err := session.Read(ctx, out, filament.Boundary{MaxRecords: 1, MaxWait: time.Second})
	if err != nil || len(c.Positions) != 1 || len(out.Records) != 0 {
		t.Fatalf("empty snapshot coverage=%v rows=%v err=%v", c, out.Records, err)
	}
	if err := session.Acknowledge(ctx, c); err != nil {
		t.Fatal(err)
	}
}
