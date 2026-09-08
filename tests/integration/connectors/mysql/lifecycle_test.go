//go:build integration

package mysql_test

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	mysqlsink "github.com/galaxy-io/filament/connectors/mysql/sink"
	mysqlsource "github.com/galaxy-io/filament/connectors/mysql/source"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

func TestMySQLConnectorLifecycle(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()
	db := testcontainers.MySQLContainer(t)

	t.Run("typed snapshot to sink", func(t *testing.T) {
		if _, err := db.DB.ExecContext(ctx, `
			CREATE TABLE accounts (
				id bigint unsigned PRIMARY KEY,
				code varbinary(8) NOT NULL,
				name varchar(80) NOT NULL,
				amount decimal(20,4) NOT NULL,
				active boolean NOT NULL,
				updated_at datetime(6) NOT NULL,
				metadata json NOT NULL,
				note text NULL
			);
			INSERT INTO accounts VALUES
				(1, X'00FF10', 'Ada', 1234567890123456.1250, TRUE, '2026-08-01 12:34:56.123456', JSON_OBJECT('tier', 'gold'), NULL),
				(18446744073709551614, X'414243', 'Grace', -42.5000, FALSE, '2026-08-02 01:02:03.000004', JSON_OBJECT('tier', 'silver'), 'present');
		`); err != nil {
			t.Fatal(err)
		}

		src := mysqlsource.New()
		cfg := filament.NewConfig(map[string]any{"dsn": db.DSN, "page_size": 1})
		if err := src.TestConnection(ctx, cfg); err != nil {
			t.Fatalf("source connection: %v", err)
		}
		if err := src.Configure(ctx, cfg); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = src.Teardown(ctx) }()
		schema, err := src.Schema(ctx, "accounts")
		if err != nil {
			t.Fatal(err)
		}
		if len(schema.Fields) != 8 || !slices.Equal(schema.PrimaryKey, []string{"id"}) {
			t.Fatalf("accounts schema = %#v", schema)
		}
		plan, err := src.PlanResume(ctx, []string{"accounts"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		rows := &testutil.CollectSink{}
		defer rows.Release()
		if err := src.ExtractFrom(ctx, rows, filament.ExtractOpts{Resources: []string{"accounts"}, Parallelism: 2}, plan); err != nil {
			t.Fatal(err)
		}
		gotIDs := make([]string, 0, 2)
		for _, row := range testutil.DataRecords(rows.Records) {
			gotIDs = append(gotIDs, row.ID)
		}
		if want := []string{"1", "18446744073709551614"}; !slices.Equal(gotIDs, want) {
			t.Fatalf("snapshot ids = %v, want %v", gotIDs, want)
		}

		policy := filament.WritePolicyForIngestion(filament.IngestionFullReplace)
		policy.Resource, policy.Keys = "accounts", []string{"id"}
		dst := mysqlsink.New()
		run := filament.RunSpec{
			Run:           "mysql-typed-snapshot",
			Resources:     []string{"accounts"},
			Sink:          filament.Ref{Config: map[string]any{"dsn": db.DSN, "database": "sinkdb"}},
			WritePolicies: map[string]filament.WritePolicy{"accounts": policy},
		}
		if err := dst.TestConnection(ctx, filament.NewConfig(run.Sink.Config)); err != nil {
			t.Fatalf("sink connection: %v", err)
		}
		if err := dst.Open(ctx, run); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = dst.Abort(ctx) }()
		if err := dst.EnsureSchema(ctx, "accounts", schema); err != nil {
			t.Fatal(err)
		}
		if err := testutil.ApplyAll(ctx, dst, rows.BatchesFor("accounts"), policy); err != nil {
			t.Fatal(err)
		}
		if err := dst.Commit(ctx); err != nil {
			t.Fatal(err)
		}

		query := `SELECT CAST(id AS CHAR), HEX(code), name, CAST(amount AS CHAR), active,
			DATE_FORMAT(updated_at, '%Y-%m-%d %H:%i:%s.%f'), JSON_UNQUOTE(JSON_EXTRACT(metadata, '$.tier')), note IS NULL
			FROM sinkdb.accounts ORDER BY id`
		result, err := db.RootDB.QueryContext(ctx, query)
		if err != nil {
			t.Fatal(err)
		}
		defer result.Close()
		var got []string
		for result.Next() {
			var id, code, name, amount, stamp, tier string
			var active, noteNull bool
			if err := result.Scan(&id, &code, &name, &amount, &active, &stamp, &tier, &noteNull); err != nil {
				t.Fatal(err)
			}
			got = append(got, fmt.Sprintf("%s|%s|%s|%s|%t|%s|%s|%t", id, code, name, amount, active, stamp, tier, noteNull))
		}
		if err := result.Err(); err != nil {
			t.Fatal(err)
		}
		want := []string{
			"1|00FF10|Ada|1234567890123456.1250|true|2026-08-01 12:34:56.123456|gold|true",
			"18446744073709551614|414243|Grace|-42.5000|false|2026-08-02 01:02:03.000004|silver|false",
		}
		if !slices.Equal(got, want) {
			t.Fatalf("destination rows = %#v, want %#v", got, want)
		}
	})

	t.Run("parallel keyset has no gaps or duplicates", func(t *testing.T) {
		if _, err := db.DB.ExecContext(ctx, `
			CREATE TABLE sharded_rows (id bigint PRIMARY KEY, payload varchar(512) NOT NULL);
			INSERT INTO sharded_rows
			SELECT n, REPEAT(CONCAT('payload-', n), 32)
			FROM JSON_TABLE(CONCAT('[', REPEAT('0,', 3999), '0]'), '$[*]' COLUMNS(n FOR ORDINALITY)) AS seq;
			ANALYZE TABLE sharded_rows;
		`); err != nil {
			t.Fatal(err)
		}
		src := mysqlsource.New()
		if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": db.DSN, "page_size": 97, "shard_pages": 1, "max_conns": 8})); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = src.Teardown(ctx) }()
		plan, err := src.PlanResume(ctx, []string{"sharded_rows"}, nil)
		if err != nil {
			t.Fatal(err)
		}
		ks, ok := checkpoint.ParseKeyset(plan["sharded_rows"])
		if !ok || len(ks.Shards) < 2 {
			t.Fatalf("expected multiple keyset shards, got %#v", plan["sharded_rows"])
		}
		out := &testutil.CollectSink{}
		defer out.Release()
		if err := src.ExtractFrom(ctx, out, filament.ExtractOpts{Resources: []string{"sharded_rows"}, Parallelism: 8}, plan); err != nil {
			t.Fatal(err)
		}
		seen := make(map[string]struct{}, 4000)
		for _, row := range testutil.DataRecords(out.Records) {
			if _, duplicate := seen[row.ID]; duplicate {
				t.Fatalf("duplicate key %s across shards", row.ID)
			}
			seen[row.ID] = struct{}{}
		}
		if len(seen) != 4000 {
			t.Fatalf("distinct rows = %d, want 4000", len(seen))
		}
		for _, id := range []string{"1", "2000", "4000"} {
			if _, ok := seen[id]; !ok {
				t.Fatalf("boundary id %s was not extracted", id)
			}
		}
	})

	t.Run("incremental backfill and delta", func(t *testing.T) {
		if _, err := db.DB.ExecContext(ctx, `
			CREATE TABLE incremental_rows (id bigint PRIMARY KEY, name varchar(80) NOT NULL, updated_at datetime(6) NOT NULL);
			INSERT INTO incremental_rows
			SELECT n, CONCAT('row-', n), TIMESTAMP('2026-08-01 00:00:00') + INTERVAL n SECOND
			FROM JSON_TABLE(CONCAT('[', REPEAT('0,', 999), '0]'), '$[*]' COLUMNS(n FOR ORDINALITY)) AS seq;
		`); err != nil {
			t.Fatal(err)
		}
		src := mysqlsource.New()
		if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": db.DSN, "page_size": 101, "shard_pages": 1})); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = src.Teardown(ctx) }()
		cursors := map[string]filament.ResourceCursorConfig{"incremental_rows": {Field: "updated_at"}}
		plan, err := src.PlanIncremental(ctx, []string{"incremental_rows"}, nil, cursors)
		if err != nil {
			t.Fatal(err)
		}
		initial := &testutil.CollectSink{}
		defer initial.Release()
		if err := src.ExtractFrom(ctx, initial, filament.ExtractOpts{Resources: []string{"incremental_rows"}, Parallelism: 4}, plan); err != nil {
			t.Fatal(err)
		}
		if got := len(testutil.DataRecords(initial.Records)); got != 1000 {
			t.Fatalf("incremental backfill rows = %d, want 1000", got)
		}
		promoted, ok := checkpoint.PromoteIncrementalBackfill(plan["incremental_rows"])
		if !ok {
			t.Fatal("incremental backfill checkpoint did not promote")
		}
		if _, err := db.DB.ExecContext(ctx, `
			UPDATE incremental_rows SET name='changed', updated_at='2026-08-03 00:00:00.000001' WHERE id=1;
			INSERT INTO incremental_rows VALUES (1001, 'new', '2026-08-03 00:00:00.000002');
		`); err != nil {
			t.Fatal(err)
		}
		next, err := src.PlanIncremental(ctx, []string{"incremental_rows"}, map[string]filament.Checkpoint{"incremental_rows": promoted}, cursors)
		if err != nil {
			t.Fatal(err)
		}
		delta := &testutil.CollectSink{}
		defer delta.Release()
		if err := src.ExtractFrom(ctx, delta, filament.ExtractOpts{Resources: []string{"incremental_rows"}, Parallelism: 2}, next); err != nil {
			t.Fatal(err)
		}
		var ids []string
		for _, row := range testutil.DataRecords(delta.Records) {
			ids = append(ids, row.ID)
		}
		if want := []string{"1", "1001"}; !slices.Equal(ids, want) {
			t.Fatalf("incremental ids = %v, want %v", ids, want)
		}
	})

	t.Run("GTID CDC preserves operation order", func(t *testing.T) {
		if _, err := db.DB.ExecContext(ctx, `CREATE TABLE cdc_rows (id bigint PRIMARY KEY, name varchar(80) NOT NULL)`); err != nil {
			t.Fatal(err)
		}
		src := mysqlsource.New()
		if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": db.DSN, "replication": "cdc", "server_id": 62348})); err != nil {
			t.Fatal(err)
		}
		defer func() { _ = src.Teardown(ctx) }()
		initial := &testutil.CollectSink{}
		defer initial.Release()
		if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{"cdc_rows"}}); err != nil {
			t.Fatal(err)
		}
		first := testutil.StreamCheckpoint(t, initial.Records, "cdc_rows")
		lsn, _, ok := checkpoint.ParseStream(first)
		if !ok || !strings.HasPrefix(lsn, "gtid:") {
			t.Fatalf("initial cursor = %q, want GTID checkpoint", lsn)
		}
		for _, statement := range []string{
			"INSERT INTO cdc_rows VALUES (2, 'Ada')",
			"UPDATE cdc_rows SET name='Grace' WHERE id=2",
			"UPDATE cdc_rows SET id=3 WHERE id=2",
			"DELETE FROM cdc_rows WHERE id=3",
		} {
			if _, err := db.DB.ExecContext(ctx, statement); err != nil {
				t.Fatal(err)
			}
		}
		changes := &testutil.CollectSink{}
		defer changes.Release()
		if err := src.ExtractChanges(ctx, changes, filament.ChangeExtractOpts{
			Resources: []string{"cdc_rows"}, Checkpoints: map[string]filament.Checkpoint{"cdc_rows": first},
		}); err != nil {
			t.Fatal(err)
		}
		data := testutil.DataRecords(changes.Records)
		var got []string
		for _, row := range data {
			got = append(got, fmt.Sprintf("%s:%s", filament.OperationName(row.Op), row.ID))
			if !strings.HasPrefix(row.LSN, "gtid:") {
				t.Fatalf("row cursor %q is not GTID", row.LSN)
			}
		}
		want := []string{"insert:2", "update:2", "delete:2", "insert:3", "delete:3"}
		if !slices.Equal(got, want) {
			t.Fatalf("CDC records = %v, want %v", got, want)
		}
		next := testutil.StreamCheckpoint(t, changes.Records, "cdc_rows")
		nextLSN, _, ok := checkpoint.ParseStream(next)
		if !ok || nextLSN == lsn {
			t.Fatalf("CDC checkpoint did not advance: before=%q after=%q", lsn, nextLSN)
		}
	})
}
