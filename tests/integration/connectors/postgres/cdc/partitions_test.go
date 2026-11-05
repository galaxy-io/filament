//go:build integration

package cdc_test

import (
	"context"
	"slices"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

// createEvents makes a range-partitioned table whose October partition is
// itself list-partitioned by region, with three rows in each leaf: ids 1-3 in
// September, 4-6 in October EU, 7-9 in October US.
func createEvents(t *testing.T, pg *testcontainers.PG) {
	t.Helper()
	if _, err := pg.Pool().Exec(context.Background(), `
		CREATE TABLE events (id bigint, at date NOT NULL, region text NOT NULL, note text, PRIMARY KEY (id, at, region)) PARTITION BY RANGE (at);
		CREATE TABLE events_2026_09 PARTITION OF events FOR VALUES FROM ('2026-09-01') TO ('2026-10-01');
		CREATE TABLE events_2026_10 PARTITION OF events FOR VALUES FROM ('2026-10-01') TO ('2026-11-01') PARTITION BY LIST (region);
		CREATE TABLE events_2026_10_eu PARTITION OF events_2026_10 FOR VALUES IN ('eu');
		CREATE TABLE events_2026_10_us PARTITION OF events_2026_10 FOR VALUES IN ('us');
		INSERT INTO events VALUES
			(1, '2026-09-02', 'eu', 'a'), (2, '2026-09-03', 'us', 'b'), (3, '2026-09-04', 'eu', 'c'),
			(4, '2026-10-02', 'eu', 'd'), (5, '2026-10-03', 'eu', 'e'), (6, '2026-10-04', 'eu', 'f'),
			(7, '2026-10-02', 'us', 'g'), (8, '2026-10-03', 'us', 'h'), (9, '2026-10-04', 'us', 'i');
	`); err != nil {
		t.Fatal(err)
	}
}

// eventIDs returns the id of every data record, in order, checking each one
// arrived under the partitioned table's name.
func eventIDs(t *testing.T, records []testutil.Record) []string {
	t.Helper()
	var ids []string
	for _, record := range testutil.DataRecords(records) {
		if record.Resource != "events" {
			t.Fatalf("record emitted under %q, want events", record.Resource)
		}
		ids = append(ids, strings.Split(record.ID, "\x1f")[0])
	}
	return ids
}

func TestPostgresPartitionedTableReadsItsPartitions(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	createEvents(t, pg)
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "shard_pages": 1})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()

	discovered, err := src.Discover(ctx, filament.DiscoverOpts{})
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, resource := range discovered.Resources {
		names = append(names, resource.Name)
	}
	if !slices.Equal(names, []string{"events"}) {
		t.Fatalf("discovered %v, want the partitioned table alone", names)
	}
	partitions, err := src.Partitions(ctx, "events")
	if err != nil {
		t.Fatal(err)
	}
	if want := []string{"events_2026_09", "events_2026_10_eu", "events_2026_10_us"}; !slices.Equal(partitions, want) {
		t.Fatalf("partitions = %v, want %v", partitions, want)
	}

	all := &testutil.CollectSink{}
	if err := src.Extract(ctx, all, filament.ExtractOpts{Resources: []string{"events"}, Parallelism: 4}); err != nil {
		t.Fatal(err)
	}
	full := eventIDs(t, all.Records)
	slices.Sort(full)
	if !slices.Equal(full, []string{"1", "2", "3", "4", "5", "6", "7", "8", "9"}) {
		t.Fatalf("full read = %v, want ids 1 through 9", full)
	}

	if err := src.SelectPartitions(map[string][]string{"events": {"events_2026_10_eu"}}); err != nil {
		t.Fatal(err)
	}
	plan, err := src.PlanResume(ctx, []string{"events"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	if plan["events"] != nil {
		t.Fatalf("a narrowed table must read through the partition-aware reader, got plan %v", plan["events"].Raw())
	}
	narrowed := &testutil.CollectSink{}
	if err := src.Extract(ctx, narrowed, filament.ExtractOpts{Resources: []string{"events"}}); err != nil {
		t.Fatal(err)
	}
	got := eventIDs(t, narrowed.Records)
	slices.Sort(got)
	if !slices.Equal(got, []string{"4", "5", "6"}) {
		t.Fatalf("narrowed read = %v, want the October EU rows", got)
	}

	if err := src.SelectPartitions(map[string][]string{"events": {}}); err != nil {
		t.Fatal(err)
	}
	none := &testutil.CollectSink{}
	if err := src.Extract(ctx, none, filament.ExtractOpts{Resources: []string{"events"}}); err != nil {
		t.Fatal(err)
	}
	if got := eventIDs(t, none.Records); len(got) != 0 {
		t.Fatalf("an empty selection must read no rows, got %v", got)
	}

	if err := src.SelectPartitions(map[string][]string{"events": {"events_2026_12"}}); err != nil {
		t.Fatal(err)
	}
	if err := src.Extract(ctx, &testutil.CollectSink{}, filament.ExtractOpts{Resources: []string{"events"}}); err == nil || !strings.Contains(err.Error(), "no partition") {
		t.Fatalf("an unknown partition must fail the read: %v", err)
	}

	if _, err := pg.Pool().Exec(ctx, `CREATE TABLE plain (id bigint PRIMARY KEY)`); err != nil {
		t.Fatal(err)
	}
	if err := src.SelectPartitions(map[string][]string{"plain": {"plain_1"}}); err != nil {
		t.Fatal(err)
	}
	if err := src.Extract(ctx, &testutil.CollectSink{}, filament.ExtractOpts{Resources: []string{"plain"}}); err == nil || !strings.Contains(err.Error(), "not partitioned") {
		t.Fatalf("a selection on a plain table must fail the read: %v", err)
	}
}

func TestPostgresCDCPartitionedTableBackfillsSelectedPartitions(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	createEvents(t, pg)
	// A publication made before partition support names leaf partitions in the
	// WAL until the stream turns publish_via_partition_root on.
	if _, err := pg.Pool().Exec(ctx, `CREATE PUBLICATION filament_test_partitions FOR TABLE events WITH (publish = 'insert, update, delete')`); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "slot_name": "filament_test_partitions"})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	if err := src.SelectPartitions(map[string][]string{"events": {"events_2026_10_eu"}}); err != nil {
		t.Fatal(err)
	}

	initial := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{"events"}}); err != nil {
		t.Fatal(err)
	}
	got := eventIDs(t, initial.Records)
	slices.Sort(got)
	if !slices.Equal(got, []string{"4", "5", "6"}) {
		t.Fatalf("backfill = %v, want the October EU rows", got)
	}
	var viaRoot bool
	var tables []string
	if err := pg.Pool().QueryRow(ctx, `SELECT pubviaroot, (SELECT array_agg(tablename::text) FROM pg_publication_tables WHERE pubname = $1) FROM pg_publication WHERE pubname = $1`, "filament_test_partitions").Scan(&viaRoot, &tables); err != nil {
		t.Fatal(err)
	}
	if !viaRoot || !slices.Equal(tables, []string{"events"}) {
		t.Fatalf("publication via root = %v tables = %v, want the partitioned table published via its root", viaRoot, tables)
	}
	first := testutil.StreamCheckpoint(t, initial.Records, "events")

	// Changes reach the stream from every partition, including one the backfill
	// skipped and one created after the stream started.
	if _, err := pg.Pool().Exec(ctx, `
		INSERT INTO events VALUES (10, '2026-09-10', 'eu', 'j');
		UPDATE events SET note = 'd2' WHERE id = 4;
		DELETE FROM events WHERE id = 7;
		CREATE TABLE events_2026_11 PARTITION OF events FOR VALUES FROM ('2026-11-01') TO ('2026-12-01');
		INSERT INTO events VALUES (11, '2026-11-02', 'us', 'k');
	`); err != nil {
		t.Fatal(err)
	}
	changes := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, changes, filament.ChangeExtractOpts{Resources: []string{"events"}, Checkpoints: map[string]filament.Checkpoint{"events": first}}); err != nil {
		t.Fatal(err)
	}
	records := testutil.DataRecords(changes.Records)
	if ids := eventIDs(t, changes.Records); !slices.Equal(ids, []string{"10", "4", "7", "11"}) {
		t.Fatalf("WAL ids = %v, want 10, 4, 7, 11", ids)
	}
	if ops := []filament.Operation{records[0].Op, records[1].Op, records[2].Op, records[3].Op}; !slices.Equal(ops, []filament.Operation{filament.OpInsert, filament.OpUpdate, filament.OpDelete, filament.OpInsert}) {
		t.Fatalf("WAL ops = %v, want insert, update, delete, insert", ops)
	}
}

func TestPostgresCDCPartitionColumnListFollowsLeafIdentity(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	createEvents(t, pg)
	// A leaf's own replica identity governs its updates, even when the
	// partitioned parent keeps the default.
	if _, err := pg.Pool().Exec(ctx, `ALTER TABLE events_2026_10_us REPLICA IDENTITY FULL`); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "slot_name": "filament_test_partition_identity"})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	if err := src.SelectColumns(map[string][]string{"events": {"id", "at", "region"}}); err != nil {
		t.Fatal(err)
	}
	initial := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{"events"}}); err != nil {
		t.Fatal(err)
	}
	var viaRoot bool
	if err := pg.Pool().QueryRow(ctx, `SELECT pubviaroot FROM pg_publication WHERE pubname = $1`, "filament_test_partition_identity").Scan(&viaRoot); err != nil {
		t.Fatal(err)
	}
	if !viaRoot {
		t.Fatal("a created publication must publish partitioned tables via their root")
	}
	if got := publishedColumns(t, pg, "filament_test_partition_identity", "events"); strings.Join(got, ",") != "id,at,region,note" {
		t.Fatalf("publication columns = %v, want every column while a leaf has REPLICA IDENTITY FULL", got)
	}
	if _, err := pg.Pool().Exec(ctx, `UPDATE events SET note = 'g2' WHERE id = 7`); err != nil {
		t.Fatalf("the application's update on the FULL leaf must still succeed: %v", err)
	}

	// Once every leaf's identity is covered, the root publishes the selection.
	if _, err := pg.Pool().Exec(ctx, `ALTER TABLE events_2026_10_us REPLICA IDENTITY DEFAULT`); err != nil {
		t.Fatal(err)
	}
	if err := src.ExtractChanges(ctx, &testutil.CollectSink{}, filament.ChangeExtractOpts{Resources: []string{"events"}, Checkpoints: map[string]filament.Checkpoint{"events": testutil.StreamCheckpoint(t, initial.Records, "events")}}); err != nil {
		t.Fatal(err)
	}
	if got := publishedColumns(t, pg, "filament_test_partition_identity", "events"); strings.Join(got, ",") != "id,at,region" {
		t.Fatalf("publication columns = %v, want the selection", got)
	}
	if _, err := pg.Pool().Exec(ctx, `UPDATE events SET note = 'g3' WHERE id = 7`); err != nil {
		t.Fatalf("an update under the narrowed column list must succeed: %v", err)
	}
}

func TestPostgresCDCAdministeredPublicationNeedsPartitionRoot(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	createEvents(t, pg)
	if _, err := pg.Pool().Exec(ctx, `CREATE PUBLICATION shared_partitions FOR TABLE events`); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "slot_name": "filament_test_shared_partitions", "manage_publication": false, "publication": "shared_partitions",
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	err := src.ExtractChanges(ctx, &testutil.CollectSink{}, filament.ChangeExtractOpts{Resources: []string{"events"}})
	if err == nil || !strings.Contains(err.Error(), "publish_via_partition_root") {
		t.Fatalf("an administered publication without publish_via_partition_root must be rejected: %v", err)
	}
}

func TestPostgresCDCPartitionedTableWithoutBackfillStreamsChanges(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	createEvents(t, pg)
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN(), "slot_name": "filament_test_partitions_none"})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	if err := src.SelectPartitions(map[string][]string{"events": {}}); err != nil {
		t.Fatal(err)
	}
	initial := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{"events"}}); err != nil {
		t.Fatal(err)
	}
	if got := eventIDs(t, initial.Records); len(got) != 0 {
		t.Fatalf("backfill = %v, want none", got)
	}
	first := testutil.StreamCheckpoint(t, initial.Records, "events")
	if _, err := pg.Pool().Exec(ctx, `INSERT INTO events VALUES (10, '2026-10-10', 'eu', 'j')`); err != nil {
		t.Fatal(err)
	}
	changes := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, changes, filament.ChangeExtractOpts{Resources: []string{"events"}, Checkpoints: map[string]filament.Checkpoint{"events": first}}); err != nil {
		t.Fatal(err)
	}
	if ids := eventIDs(t, changes.Records); !slices.Equal(ids, []string{"10"}) {
		t.Fatalf("WAL ids = %v, want 10", ids)
	}
}
