//go:build integration

package sqlserver_test

import (
	"cmp"
	"context"
	"fmt"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	source "github.com/galaxy-io/filament/connectors/sqlserver/source"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	"github.com/galaxy-io/filament/tests/testcontainers"
)

func TestSQLServerCDC(t *testing.T) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	ms := testcontainers.MSSQLContainer(t)
	exec := func(t *testing.T, stmts ...string) {
		t.Helper()
		for _, stmt := range stmts {
			if _, err := ms.DB.ExecContext(ctx, stmt); err != nil {
				t.Fatalf("%s: %v", stmt, err)
			}
		}
	}
	configure := func(t *testing.T, extra map[string]any) *source.Source {
		t.Helper()
		cfg := map[string]any{"connection_method": "url", "dsn": ms.DSN, "replication": "cdc"}
		for k, v := range extra {
			cfg[k] = v
		}
		src := source.New()
		if err := src.Configure(ctx, filament.NewConfig(cfg)); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = src.Teardown(ctx) })
		return src
	}
	// captured waits until capture has copied every transaction committed so
	// far: it commits a marker and waits for the marker's change row. Capture
	// reads the log in commit order.
	var marks int
	captured := func(t *testing.T) {
		t.Helper()
		marks++
		exec(t, fmt.Sprintf("INSERT INTO dbo.marker VALUES (%d)", marks))
		deadline := time.Now().Add(2 * time.Minute)
		for {
			var n int
			if err := ms.DB.QueryRowContext(ctx, "SELECT COUNT(*) FROM cdc.dbo_marker_CT WHERE id = @p1", marks).Scan(&n); err != nil {
				t.Fatal(err)
			}
			if n > 0 {
				return
			}
			if time.Now().After(deadline) {
				t.Fatal("capture did not reach the marker")
			}
			time.Sleep(250 * time.Millisecond)
		}
	}
	extract := func(t *testing.T, src *source.Source, opts filament.ChangeExtractOpts) *testutil.CollectSink {
		t.Helper()
		out := &testutil.CollectSink{}
		t.Cleanup(out.Release)
		if err := src.ExtractChanges(ctx, out, opts); err != nil {
			t.Fatal(err)
		}
		return out
	}
	checkpoints := func(t *testing.T, out *testutil.CollectSink, resources ...string) map[string]filament.Checkpoint {
		t.Helper()
		cps := map[string]filament.Checkpoint{}
		for _, r := range resources {
			cps[r] = testutil.StreamCheckpoint(t, out.Records, r)
		}
		return cps
	}
	type change struct {
		resource, id string
		op           filament.Operation
	}
	// changes lists the data rows in emission order.
	changes := func(out *testutil.CollectSink) []change {
		data := slices.Clone(testutil.DataRecords(out.Records))
		slices.SortStableFunc(data, func(a, b testutil.Record) int { return cmp.Compare(a.Seq, b.Seq) })
		var got []change
		for _, r := range data {
			got = append(got, change{r.Resource, r.ID, r.Op})
		}
		return got
	}

	exec(t,
		"EXEC sys.sp_cdc_enable_db",
		"CREATE TABLE dbo.marker (id int PRIMARY KEY)",
		"EXEC sys.sp_cdc_enable_table @source_schema = N'dbo', @source_name = N'marker', @role_name = NULL",
		"CREATE TABLE dbo.accounts (id int PRIMARY KEY, name nvarchar(50), balance decimal(12,2))",
		"CREATE TABLE dbo.ledger (account int NOT NULL, entry varchar(10) NOT NULL, amount decimal(12,2), PRIMARY KEY (account, entry))",
		"INSERT INTO dbo.accounts VALUES (1, N'ann', 10), (2, N'bob', 20)",
		"INSERT INTO dbo.ledger VALUES (1, 'open', 10)",
	)
	resources := []string{"accounts", "ledger"}

	// The subtests run in order against one database: later ones build on the
	// tables and capture instances earlier ones leave behind, so run the test
	// as a whole rather than one subtest with -run.

	t.Run("snapshot, then changes in commit order across tables", func(t *testing.T) {
		src := configure(t, nil) // manage_capture enables both tables
		first := extract(t, src, filament.ChangeExtractOpts{Resources: resources})
		if got := changes(first); len(got) != 3 {
			t.Fatalf("snapshot rows = %v", got)
		}
		cps := checkpoints(t, first, resources...)

		exec(t,
			`BEGIN TRAN
				UPDATE dbo.accounts SET balance = 5 WHERE id = 1
				INSERT INTO dbo.ledger VALUES (1, 'w1', -5)
				UPDATE dbo.accounts SET balance = 25 WHERE id = 2
			COMMIT`,
			"UPDATE dbo.accounts SET id = 10 WHERE id = 1",
			"DELETE FROM dbo.ledger WHERE entry = 'open'",
		)
		captured(t)
		second := extract(t, src, filament.ChangeExtractOpts{Resources: resources, Checkpoints: cps})
		want := []change{
			{"accounts", "1", filament.OpUpdate},
			{"ledger", "1\x1fw1", filament.OpInsert},
			{"accounts", "2", filament.OpUpdate},
			{"accounts", "1", filament.OpDelete},
			{"accounts", "10", filament.OpInsert},
			{"ledger", "1\x1fopen", filament.OpDelete},
		}
		if got := changes(second); !slices.Equal(got, want) {
			t.Fatalf("changes = %v\nwant      %v", got, want)
		}

		idle := extract(t, src, filament.ChangeExtractOpts{Resources: resources, Checkpoints: checkpoints(t, second, resources...)})
		if got := changes(idle); len(got) != 0 {
			t.Fatalf("resumed cycle re-read %v", got)
		}
	})

	t.Run("purged changes fail instead of skipping", func(t *testing.T) {
		src := configure(t, nil)
		base := extract(t, src, filament.ChangeExtractOpts{Resources: []string{"accounts"}})
		stale := checkpoints(t, base, "accounts")
		exec(t, "UPDATE dbo.accounts SET name = N'bobby' WHERE id = 2")
		captured(t)
		exec(t, `DECLARE @lwm binary(10) = sys.fn_cdc_get_max_lsn();
			EXEC sys.sp_cdc_cleanup_change_table @capture_instance = N'dbo_accounts', @low_water_mark = @lwm, @threshold = 100000`)
		out := &testutil.CollectSink{}
		defer out.Release()
		err := src.ExtractChanges(ctx, out, filament.ChangeExtractOpts{Resources: []string{"accounts"}, Checkpoints: stale})
		if err == nil || !strings.Contains(err.Error(), "no longer in capture instance") {
			t.Fatalf("stale checkpoint: %v", err)
		}
	})

	t.Run("a second capture instance takes over without a gap", func(t *testing.T) {
		exec(t, "CREATE TABLE dbo.items (id int PRIMARY KEY, a int)")
		src := configure(t, nil)
		base := extract(t, src, filament.ChangeExtractOpts{Resources: []string{"items"}})
		// The insert lands before b exists, so only the old instance holds
		// it, without a value for b, and the run fails rather than guess.
		exec(t,
			"INSERT INTO dbo.items VALUES (1, 1)",
			"ALTER TABLE dbo.items ADD b int",
			"EXEC sys.sp_cdc_enable_table @source_schema = N'dbo', @source_name = N'items', @role_name = NULL, @capture_instance = N'dbo_items_v2'",
		)
		captured(t)
		failed := &testutil.CollectSink{}
		defer failed.Release()
		err := src.ExtractChanges(ctx, failed, filament.ChangeExtractOpts{Resources: []string{"items"}, Checkpoints: checkpoints(t, base, "items")})
		if err == nil || !strings.Contains(err.Error(), `does not capture "b"`) {
			t.Fatalf("change without b: %v", err)
		}
		// A new snapshot starts the table past that change; later changes
		// come from the new instance.
		base = extract(t, src, filament.ChangeExtractOpts{Resources: []string{"items"}})
		exec(t, "INSERT INTO dbo.items VALUES (2, 2, 2)")
		captured(t)
		out := extract(t, src, filament.ChangeExtractOpts{Resources: []string{"items"}, Checkpoints: checkpoints(t, base, "items")})
		rows := renderRows(t, out.BatchesFor("items"))
		var got []string
		for _, r := range rows {
			got = append(got, r["id"]+":"+r["b"])
		}
		if !slices.Equal(got, []string{"2:2"}) {
			t.Fatalf("rows across instances = %v", got)
		}
		mark, _, _ := checkpoint.ParseStream(testutil.StreamCheckpoint(t, out.Records, "items"))
		var v2Start string
		if err := ms.DB.QueryRowContext(ctx, "SELECT CONVERT(varchar(20), start_lsn, 2) FROM cdc.change_tables WHERE capture_instance = N'dbo_items_v2'").Scan(&v2Start); err != nil {
			t.Fatal(err)
		}
		if strings.ToLower(v2Start) > mark {
			t.Fatalf("mark %s did not reach the new instance's start %s", mark, v2Start)
		}
	})

	t.Run("an idle capture does not hold up the snapshot", func(t *testing.T) {
		exec(t,
			"CREATE TABLE dbo.quiet (id int PRIMARY KEY)",
			"INSERT INTO dbo.quiet VALUES (1)",
			"EXEC sys.sp_cdc_enable_table @source_schema = N'dbo', @source_name = N'quiet', @role_name = NULL",
			"CREATE TABLE dbo.uncaptured (id int IDENTITY PRIMARY KEY, v int)",
		)
		captured(t)
		// Let capture settle into a run of empty scans, which SQL Server keeps
		// on one session row, then move the log end past the last captured
		// change with a write capture ignores.
		time.Sleep(12 * time.Second)
		exec(t, "INSERT INTO dbo.uncaptured (v) VALUES (1)")
		src := configure(t, nil)
		started := time.Now()
		out := extract(t, src, filament.ChangeExtractOpts{Resources: []string{"quiet"}})
		if elapsed := time.Since(started); elapsed > time.Minute {
			t.Fatalf("snapshot waited %s for an idle capture", elapsed)
		}
		if n := len(testutil.DataRecords(out.Records)); n != 1 {
			t.Fatalf("snapshot rows = %d, want 1", n)
		}
	})

	t.Run("computed columns are left out", func(t *testing.T) {
		exec(t,
			"CREATE TABLE dbo.stock (id int PRIMARY KEY, qty int NOT NULL, doubled AS qty * 2 PERSISTED)",
			"INSERT INTO dbo.stock (id, qty) VALUES (1, 5)",
		)
		src := configure(t, nil)
		base := extract(t, src, filament.ChangeExtractOpts{Resources: []string{"stock"}})
		exec(t, "UPDATE dbo.stock SET qty = 6 WHERE id = 1")
		captured(t)
		out := extract(t, src, filament.ChangeExtractOpts{Resources: []string{"stock"}, Checkpoints: checkpoints(t, base, "stock")})
		for _, rows := range [][]map[string]string{renderRows(t, base.BatchesFor("stock")), renderRows(t, out.BatchesFor("stock"))} {
			if len(rows) != 1 {
				t.Fatalf("rows = %v, want one", rows)
			}
			if _, ok := rows[0]["doubled"]; ok {
				t.Fatalf("row %v carries the computed column", rows[0])
			}
		}
		if got := renderRows(t, out.BatchesFor("stock"))[0]["qty"]; got != "6" {
			t.Fatalf("updated qty = %s, want 6", got)
		}

		// A computed key column cannot be left out without merging rows.
		exec(t, "CREATE TABLE dbo.slots (tenant int NOT NULL, raw int NOT NULL, slot AS raw * 10 PERSISTED NOT NULL, PRIMARY KEY (tenant, slot))")
		rejected := &testutil.CollectSink{}
		defer rejected.Release()
		if err := src.ExtractChanges(ctx, rejected, filament.ChangeExtractOpts{Resources: []string{"slots"}}); err == nil || !strings.Contains(err.Error(), `primary key column "slot" is computed`) {
			t.Fatalf("computed key column: %v", err)
		}
	})

	t.Run("commit order holds while an instance rotates", func(t *testing.T) {
		exec(t,
			"CREATE TABLE dbo.rot_a (id int PRIMARY KEY)",
			"CREATE TABLE dbo.rot_b (id int PRIMARY KEY)",
		)
		pair := []string{"rot_a", "rot_b"}
		src := configure(t, nil)
		base := extract(t, src, filament.ChangeExtractOpts{Resources: pair})
		exec(t, "EXEC sys.sp_cdc_enable_table @source_schema = N'dbo', @source_name = N'rot_a', @role_name = NULL, @capture_instance = N'dbo_rot_a_v2'")
		// rot_a's change is after the new instance starts, so the old one
		// cannot supply it; rot_b's commits later still.
		exec(t, "INSERT INTO dbo.rot_a VALUES (1)", "INSERT INTO dbo.rot_b VALUES (1)")
		captured(t)
		out := extract(t, src, filament.ChangeExtractOpts{Resources: pair, Checkpoints: checkpoints(t, base, pair...)})
		want := []change{{"rot_a", "1", filament.OpInsert}, {"rot_b", "1", filament.OpInsert}}
		if got := changes(out); !slices.Equal(got, want) {
			t.Fatalf("changes = %v, want %v", got, want)
		}
	})

	t.Run("commit order holds across queries of a wide selection", func(t *testing.T) {
		// Five tables of 1,000 columns are wider than one SELECT allows, so
		// they are read by two queries.
		wide := []string{"w1", "w2", "w3", "w4", "w5"}
		for _, name := range wide {
			exec(t, fmt.Sprintf(`DECLARE @sql nvarchar(max) = N'CREATE TABLE dbo.%s (id int PRIMARY KEY' + (
				SELECT STRING_AGG(CAST(CONCAT(N', c', n, N' int') AS nvarchar(max)), N'')
				FROM (SELECT TOP (999) ROW_NUMBER() OVER (ORDER BY (SELECT NULL)) AS n FROM sys.all_objects) x
			) + N')'; EXEC (@sql)`, name))
		}
		src := configure(t, nil)
		base := extract(t, src, filament.ChangeExtractOpts{Resources: wide})
		order := []string{"w4", "w1", "w5", "w2", "w4", "w3"}
		var want []change
		for i, name := range order {
			exec(t, fmt.Sprintf("INSERT INTO dbo.%s (id, c1) VALUES (%d, %d)", name, i, i))
			want = append(want, change{name, fmt.Sprint(i), filament.OpInsert})
		}
		captured(t)
		out := extract(t, src, filament.ChangeExtractOpts{Resources: wide, Checkpoints: checkpoints(t, base, wide...)})
		if got := changes(out); !slices.Equal(got, want) {
			t.Fatalf("changes = %v, want %v", got, want)
		}
	})
}
