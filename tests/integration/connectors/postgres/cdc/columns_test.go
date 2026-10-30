//go:build integration

package cdc_test

import (
	"context"
	"strings"
	"testing"

	"github.com/galaxy-io/filament"
	pgsource "github.com/galaxy-io/filament/connectors/postgres/source"
	"github.com/galaxy-io/filament/tests/internal/testutil"
	testcontainers "github.com/galaxy-io/filament/tests/testcontainers"
)

// batchColumns returns the column names every batch of resource carries.
func batchColumns(t *testing.T, sink *testutil.CollectSink, resource string) []string {
	t.Helper()
	batches := sink.BatchesFor(resource)
	if len(batches) == 0 {
		t.Fatalf("no batches for %q", resource)
	}
	var names []string
	for _, field := range batches[0].Rows().Schema().Fields() {
		names = append(names, field.Name)
	}
	for _, batch := range batches[1:] {
		if batch.Rows().Schema().NumFields() != len(names) {
			t.Fatalf("batch column count drifted for %q", resource)
		}
	}
	return names
}

func publishedColumns(t *testing.T, pg *testcontainers.PG, publication, table string) []string {
	t.Helper()
	var names []string
	if err := pg.Pool().QueryRow(context.Background(), `SELECT attnames::text[] FROM pg_publication_tables WHERE pubname=$1 AND tablename=$2`, publication, table).Scan(&names); err != nil {
		t.Fatal(err)
	}
	return names
}

func TestPostgresCDCColumnSelection(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	if _, err := pg.Pool().Exec(ctx, `
		CREATE TABLE cdc_columns (id bigint PRIMARY KEY, name text NOT NULL, secret text);
		INSERT INTO cdc_columns VALUES (1, 'Ada', 'hidden');
	`); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "slot_name": "filament_test_columns",
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	if err := src.SelectColumns(map[string][]string{"cdc_columns": {"id", "name"}}); err != nil {
		t.Fatal(err)
	}

	initial := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{"cdc_columns"}}); err != nil {
		t.Fatal(err)
	}
	if got := batchColumns(t, initial, "cdc_columns"); strings.Join(got, ",") != "id,name" {
		t.Fatalf("snapshot columns = %v, want id,name", got)
	}
	if got := publishedColumns(t, pg, "filament_test_columns", "cdc_columns"); strings.Join(got, ",") != "id,name" {
		t.Fatalf("publication columns = %v, want id,name", got)
	}
	first := testutil.StreamCheckpoint(t, initial.Records, "cdc_columns")

	if _, err := pg.Pool().Exec(ctx, `
		INSERT INTO cdc_columns VALUES (2, 'Grace', 'hidden');
		UPDATE cdc_columns SET name = 'Grace H' WHERE id = 2;
	`); err != nil {
		t.Fatal(err)
	}
	changes := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, changes, filament.ChangeExtractOpts{Resources: []string{"cdc_columns"}, Checkpoints: map[string]filament.Checkpoint{"cdc_columns": first}}); err != nil {
		t.Fatal(err)
	}
	records := testutil.DataRecords(changes.Records)
	if len(records) != 2 || records[0].Op != filament.OpInsert || records[1].Op != filament.OpUpdate {
		t.Fatalf("WAL records = %#v, want insert then update", records)
	}
	if got := batchColumns(t, changes, "cdc_columns"); strings.Join(got, ",") != "id,name" {
		t.Fatalf("WAL columns = %v, want id,name", got)
	}
	second := testutil.StreamCheckpoint(t, changes.Records, "cdc_columns")

	// Clearing the selection restores every column on the stream's publication.
	if err := src.SelectColumns(nil); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Pool().Exec(ctx, `INSERT INTO cdc_columns VALUES (3, 'Linus', 'shown')`); err != nil {
		t.Fatal(err)
	}
	widened := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, widened, filament.ChangeExtractOpts{Resources: []string{"cdc_columns"}, Checkpoints: map[string]filament.Checkpoint{"cdc_columns": second}}); err != nil {
		t.Fatal(err)
	}
	if got := publishedColumns(t, pg, "filament_test_columns", "cdc_columns"); strings.Join(got, ",") != "id,name,secret" {
		t.Fatalf("publication columns after widening = %v, want every column", got)
	}
	if got := batchColumns(t, widened, "cdc_columns"); strings.Join(got, ",") != "id,name,secret" {
		t.Fatalf("widened columns = %v", got)
	}
	third := testutil.StreamCheckpoint(t, widened.Records, "cdc_columns")

	// The publication is this stream's own, so a narrower selection narrows it.
	if err := src.SelectColumns(map[string][]string{"cdc_columns": {"id"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := pg.Pool().Exec(ctx, `INSERT INTO cdc_columns VALUES (4, 'Barbara', 'shown')`); err != nil {
		t.Fatal(err)
	}
	narrowed := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, narrowed, filament.ChangeExtractOpts{Resources: []string{"cdc_columns"}, Checkpoints: map[string]filament.Checkpoint{"cdc_columns": third}}); err != nil {
		t.Fatal(err)
	}
	if got := publishedColumns(t, pg, "filament_test_columns", "cdc_columns"); strings.Join(got, ",") != "id" {
		t.Fatalf("publication columns after narrowing = %v, want id", got)
	}
	if got := batchColumns(t, narrowed, "cdc_columns"); strings.Join(got, ",") != "id" {
		t.Fatalf("narrowed columns = %v", got)
	}
}

func TestPostgresCDCColumnSelectionWithReplicaIdentityFull(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	if _, err := pg.Pool().Exec(ctx, `
		CREATE TABLE cdc_full_columns (id bigint PRIMARY KEY, name text NOT NULL, secret text);
		ALTER TABLE cdc_full_columns REPLICA IDENTITY FULL;
		INSERT INTO cdc_full_columns VALUES (1, 'Ada', 'hidden');
	`); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "slot_name": "filament_test_full_columns",
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	if err := src.SelectColumns(map[string][]string{"cdc_full_columns": {"id", "name"}}); err != nil {
		t.Fatal(err)
	}
	initial := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{"cdc_full_columns"}}); err != nil {
		t.Fatal(err)
	}
	first := testutil.StreamCheckpoint(t, initial.Records, "cdc_full_columns")
	if _, err := pg.Pool().Exec(ctx, `UPDATE cdc_full_columns SET name = 'Ada L' WHERE id = 1`); err != nil {
		t.Fatal(err)
	}
	changes := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, changes, filament.ChangeExtractOpts{Resources: []string{"cdc_full_columns"}, Checkpoints: map[string]filament.Checkpoint{"cdc_full_columns": first}}); err != nil {
		t.Fatal(err)
	}
	// Postgres refuses a column list on a REPLICA IDENTITY FULL table, so the
	// publication carries every column and the decoder drops the rest.
	if got := publishedColumns(t, pg, "filament_test_full_columns", "cdc_full_columns"); strings.Join(got, ",") != "id,name,secret" {
		t.Fatalf("publication columns = %v, want every column", got)
	}
	if got := batchColumns(t, changes, "cdc_full_columns"); strings.Join(got, ",") != "id,name" {
		t.Fatalf("WAL columns = %v, want id,name", got)
	}
}

func TestPostgresCDCPublicationMustCoverSelection(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	if _, err := pg.Pool().Exec(ctx, `
		CREATE TABLE cdc_narrow (id bigint PRIMARY KEY, name text NOT NULL, secret text);
		CREATE PUBLICATION filament_test_narrow FOR TABLE cdc_narrow (id) WITH (publish = 'insert, update, delete');
	`); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = pg.Pool().Exec(context.Background(), `DROP PUBLICATION IF EXISTS filament_test_narrow`) })
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "slot_name": "filament_test_narrow", "publication": "filament_test_narrow", "manage_publication": false,
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	if err := src.SelectColumns(map[string][]string{"cdc_narrow": {"id", "name"}}); err != nil {
		t.Fatal(err)
	}
	err := src.ExtractChanges(ctx, &testutil.CollectSink{}, filament.ChangeExtractOpts{Resources: []string{"cdc_narrow"}})
	if err == nil || !strings.Contains(err.Error(), "does not publish every selected column") {
		t.Fatalf("narrower publication must be refused, got %v", err)
	}
	if got := publishedColumns(t, pg, "filament_test_narrow", "cdc_narrow"); strings.Join(got, ",") != "id" {
		t.Fatalf("the publication must stay untouched, got %v", got)
	}
}

func TestPostgresSnapshotColumnSelection(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgres(t)
	if _, err := pg.Pool().Exec(ctx, `
		CREATE TABLE snap_columns (id bigint PRIMARY KEY, name text NOT NULL, secret text);
		INSERT INTO snap_columns SELECT g, 'n-' || g, 's-' || g FROM generate_series(1, 50) g;
	`); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{"dsn": pg.DSN()})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	if err := src.SelectColumns(map[string][]string{"snap_columns": {"name", "id"}}); err != nil {
		t.Fatal(err)
	}
	plan, err := src.PlanResume(ctx, []string{"snap_columns"}, nil)
	if err != nil {
		t.Fatal(err)
	}
	sink := &testutil.CollectSink{}
	if err := src.ExtractFrom(ctx, sink, filament.ExtractOpts{Resources: []string{"snap_columns"}, Parallelism: 2}, plan); err != nil {
		t.Fatal(err)
	}
	if got := batchColumns(t, sink, "snap_columns"); strings.Join(got, ",") != "id,name" {
		t.Fatalf("snapshot columns = %v, want id,name in attribute order", got)
	}
	if len(testutil.DataRecords(sink.Records)) != 50 {
		t.Fatalf("rows = %d, want 50", len(testutil.DataRecords(sink.Records)))
	}

	if err := src.SelectColumns(map[string][]string{"snap_columns": {"name"}}); err != nil {
		t.Fatal(err)
	}
	if _, err := src.Schema(ctx, "snap_columns"); err == nil || !strings.Contains(err.Error(), "primary key") {
		t.Fatalf("dropping the key must fail the schema, got %v", err)
	}
}

func TestPostgresCDCColumnSelectionKeepsIndexIdentityWritable(t *testing.T) {
	ctx := context.Background()
	pg := testcontainers.SharedPostgresCDC(t)
	if _, err := pg.Pool().Exec(ctx, `
		CREATE TABLE cdc_index_identity (id bigint PRIMARY KEY, code text NOT NULL, secret text);
		CREATE UNIQUE INDEX cdc_index_identity_code ON cdc_index_identity (id, code);
		ALTER TABLE cdc_index_identity REPLICA IDENTITY USING INDEX cdc_index_identity_code;
		INSERT INTO cdc_index_identity VALUES (1, 'a', 'hidden');
	`); err != nil {
		t.Fatal(err)
	}
	src := pgsource.New()
	if err := src.Configure(ctx, filament.NewConfig(map[string]any{
		"dsn": pg.DSN(), "slot_name": "filament_test_index_identity",
	})); err != nil {
		t.Fatal(err)
	}
	defer func() { _ = src.Teardown(ctx) }()
	// The selection keeps the primary key but not the whole identity index.
	if err := src.SelectColumns(map[string][]string{"cdc_index_identity": {"id", "secret"}}); err != nil {
		t.Fatal(err)
	}
	initial := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, initial, filament.ChangeExtractOpts{Resources: []string{"cdc_index_identity"}}); err != nil {
		t.Fatal(err)
	}
	if got := publishedColumns(t, pg, "filament_test_index_identity", "cdc_index_identity"); strings.Join(got, ",") != "id,code,secret" {
		t.Fatalf("publication must stay whole when the selection misses an identity column, got %v", got)
	}
	// Postgres would reject this write under a column list missing "code".
	if _, err := pg.Pool().Exec(ctx, `UPDATE cdc_index_identity SET secret = 'changed' WHERE id = 1`); err != nil {
		t.Fatalf("application write blocked by the publication: %v", err)
	}
	first := testutil.StreamCheckpoint(t, initial.Records, "cdc_index_identity")
	changes := &testutil.CollectSink{}
	if err := src.ExtractChanges(ctx, changes, filament.ChangeExtractOpts{Resources: []string{"cdc_index_identity"}, Checkpoints: map[string]filament.Checkpoint{"cdc_index_identity": first}}); err != nil {
		t.Fatal(err)
	}
	if got := batchColumns(t, changes, "cdc_index_identity"); strings.Join(got, ",") != "id,secret" {
		t.Fatalf("WAL columns = %v, want id,secret", got)
	}
}
