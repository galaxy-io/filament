package motherduck

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/decimal128"
	duckdb "github.com/marcboeker/go-duckdb/v2"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/motherduck/internal/connection"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestLocalAppendIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "append.duckdb")
	schema := integrationSchema()
	policy := writePolicy(filament.WriteAppend, nil)
	sink := openLocalSink(t, ctx, path, "append-run", policy, 1, schema)

	batch := integrationBatch(t, schema, "events", 1, 2505)
	defer batch.Release()
	receipt, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy})
	if err != nil {
		t.Fatal(err)
	}
	if receipt.Rows != 2505 || receipt.Bytes == 0 || receipt.WriteCRC != batch.IntegrityCRC() {
		t.Fatalf("unexpected receipt: %+v", receipt)
	}
	extra := integrationBatch(t, schema, "events", 3000, 2)
	defer extra.Release()
	if _, err := sink.Apply(ctx, extra, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	verifyAppendRows(t, path)
}

func verifyAppendRows(t *testing.T, path string) {
	t.Helper()
	rows := queryLocal(t, path, `SELECT count(*) FROM "main"."events"`)
	if got := rows[0][0]; got != int64(2507) {
		t.Fatalf("count = %#v, want 2507", got)
	}
	rows = queryLocal(t, path, `SELECT name, amount, created_at, event_id FROM "main"."events" WHERE id = 1`)
	if rows[0][0] != "name-1" || text(rows[0][1]) != "1.25" {
		t.Fatalf("typed values = %#v", rows[0])
	}
	if got, ok := rows[0][2].(time.Time); !ok || got.UnixMicro() != 1704067200_000001 {
		t.Fatalf("timestamp = %#v", rows[0][2])
	}
	if got, want := text(rows[0][3]), testUUID(1); got != want {
		t.Fatalf("uuid = %#v, want %q", got, want)
	}
	rows = queryLocal(t, path, `SELECT name, amount, created_at, event_id FROM "main"."events" WHERE id = 10`)
	for i, value := range rows[0] {
		if value != nil {
			t.Errorf("nullable column %d = %#v, want nil", i, value)
		}
	}
}

func TestLocalReplaceIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "replace.duckdb")
	schema := integrationSchema()
	appendPolicy := writePolicy(filament.WriteAppend, nil)
	initial := openLocalSink(t, ctx, path, "initial-run", appendPolicy, 1, schema)
	applyBatch(t, ctx, initial, integrationBatch(t, schema, "events", 1, 2), appendPolicy)
	if err := initial.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	replacePolicy := writePolicy(filament.WriteReplace, nil)
	replacement := openLocalSink(t, ctx, path, "replace/run", replacePolicy, 1, schema)
	applyBatch(t, ctx, replacement, integrationBatch(t, schema, "events", 100, 3), replacePolicy)
	if err := replacement.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rows := queryLocal(t, path, `SELECT min(id), max(id), count(*) FROM "main"."events"`)
	if rows[0][0] != int64(100) || rows[0][1] != int64(102) || rows[0][2] != int64(3) {
		t.Fatalf("replacement values = %#v", rows[0])
	}
}

func TestLocalUpsertIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "upsert.duckdb")
	schema := integrationSchema()
	policy := writePolicy(filament.WriteUpsert, []string{"id"})
	initial := openLocalSink(t, ctx, path, "upsert-one", policy, 1, schema)
	applyBatch(t, ctx, initial, integrationBatch(t, schema, "events", 1, 2), policy)
	if err := initial.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	second := openLocalSink(t, ctx, path, "upsert-two", policy, 1, schema)
	applyBatch(t, ctx, second, integrationBatch(t, schema, "events", 2, 2), policy)
	if err := second.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rows := queryLocal(t, path, `SELECT id, name FROM "main"."events" ORDER BY id`)
	want := [][]any{{int64(1), "name-1"}, {int64(2), "name-2"}, {int64(3), "name-3"}}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
}

func TestLocalAppendMapsColumnsByNameIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "reordered.duckdb")
	queryLocal(t, path, `CREATE TABLE "main"."events" (
		"note" VARCHAR,
		"event_id" UUID,
		"Created_At" TIMESTAMP,
		"amount" DECIMAL(18, 2),
		"NAME" VARCHAR,
		"id" BIGINT NOT NULL
	)`)
	schema := integrationSchema()
	policy := writePolicy(filament.WriteAppend, nil)
	sink := openLocalSink(t, ctx, path, "append-run", policy, 1, schema)
	applyBatch(t, ctx, sink, integrationBatch(t, schema, "events", 1, 2), policy)
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rows := queryLocal(t, path, `SELECT id, name, amount, created_at, event_id, note FROM "main"."events" WHERE id = 1`)
	if rows[0][0] != int64(1) || rows[0][1] != "name-1" || text(rows[0][2]) != "1.25" || rows[0][5] != nil {
		t.Fatalf("mapped values = %#v", rows[0])
	}
	if got, ok := rows[0][3].(time.Time); !ok || got.UnixMicro() != 1704067200_000001 {
		t.Fatalf("timestamp = %#v", rows[0][3])
	}
	if got, want := text(rows[0][4]), testUUID(1); got != want {
		t.Fatalf("uuid = %q, want %q", got, want)
	}
}

func TestLocalUpsertRefusesKeylessTableIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "keyless.duckdb")
	schema := integrationSchema()
	appendPolicy := writePolicy(filament.WriteAppend, nil)
	initial := openLocalSink(t, ctx, path, "append-run", appendPolicy, 1, schema)
	applyBatch(t, ctx, initial, integrationBatch(t, schema, "events", 1, 2), appendPolicy)
	if err := initial.Commit(ctx); err != nil {
		t.Fatal(err)
	}

	policy := writePolicy(filament.WriteUpsert, []string{"id"})
	sink := newLocal(path)
	err := sink.Open(ctx, filament.RunSpec{
		Run:           "upsert-run",
		Sink:          filament.Ref{Config: map[string]any{"schema": "main"}},
		Options:       filament.RunOptions{SnapshotParallelism: 1},
		WritePolicies: map[string]filament.WritePolicy{"events": policy},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sink.Abort(context.Background()) })
	err = sink.EnsureSchema(ctx, "events", schema)
	if err == nil || !strings.Contains(err.Error(), "no primary key") {
		t.Fatalf("EnsureSchema error = %v, want a primary key refusal", err)
	}
}

func TestLocalUpsertMergesIntoEvolvedTableIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "evolved.duckdb")
	schema := integrationSchema()
	policy := writePolicy(filament.WriteUpsert, []string{"id"})
	initial := openLocalSink(t, ctx, path, "upsert-one", policy, 1, schema)
	applyBatch(t, ctx, initial, integrationBatch(t, schema, "events", 1, 2), policy)
	if err := initial.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	queryLocal(t, path, `ALTER TABLE "main"."events" ADD COLUMN "extra" VARCHAR`)

	second := openLocalSink(t, ctx, path, "upsert-two", policy, 1, schema)
	applyBatch(t, ctx, second, integrationBatch(t, schema, "events", 2, 2), policy)
	if err := second.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rows := queryLocal(t, path, `SELECT id, name, extra FROM "main"."events" ORDER BY id`)
	want := [][]any{{int64(1), "name-1", nil}, {int64(2), "name-2", nil}, {int64(3), "name-3", nil}}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("rows = %v, want %v", rows, want)
	}
}

func TestLocalAbortDropsStageIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "abort.duckdb")
	schema := integrationSchema()
	policy := writePolicy(filament.WriteReplace, nil)
	run := filament.RunID("abort/run")
	sink := openLocalSink(t, ctx, path, run, policy, 1, schema)
	applyBatch(t, ctx, sink, integrationBatch(t, schema, "events", 1, 3), policy)
	if err := sink.Abort(ctx); err != nil {
		t.Fatal(err)
	}
	stage := stageTableName("events", run)
	query := fmt.Sprintf(`SELECT count(*) FROM information_schema.tables WHERE table_schema = 'main' AND table_name = '%s'`, stage)
	if rows := queryLocal(t, path, query); rows[0][0] != int64(0) {
		t.Fatalf("stage %q still exists", stage)
	}
}

type timesSpec struct {
	id   int64
	us   int64
	null bool
}

func timesSchema() filament.RecordSchema {
	return filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "id", Logical: filament.LogicalInt64},
			{Name: "end_time", Logical: filament.LogicalTime, Nullable: true},
		},
		PrimaryKey: []string{"id"},
	}
}

func timesBatch(t *testing.T, schema filament.RecordSchema, specs []timesSpec) *arrowbatch.Batch {
	t.Helper()
	collector := &integrationCollector{}
	builder := arrowbatch.NewBuilder(arrowbatch.Schema(schema), nil, arrowbatch.Options{MaxRows: len(specs) + 1}, collector)
	for _, spec := range specs {
		builder.Int64(spec.id)
		if spec.null {
			builder.Null()
		} else {
			builder.Time(spec.us)
		}
		if err := builder.EndRow(rowmodel.Meta{}); err != nil {
			_ = builder.Close()
			t.Fatal(err)
		}
	}
	if err := builder.Flush(); err != nil {
		_ = builder.Close()
		t.Fatal(err)
	}
	if err := builder.Close(); err != nil {
		t.Fatal(err)
	}
	if len(collector.batches) != 1 {
		t.Fatalf("builder emitted %d batches, want 1", len(collector.batches))
	}
	collector.batches[0].Resource = "times"
	return collector.batches[0]
}

func openTimesSink(t *testing.T, ctx context.Context, path string, run filament.RunID, policy filament.WritePolicy, schema filament.RecordSchema) *Sink {
	t.Helper()
	sink := newLocal(path)
	err := sink.Open(ctx, filament.RunSpec{
		Run:           run,
		Sink:          filament.Ref{Config: map[string]any{"schema": "main"}},
		Options:       filament.RunOptions{SnapshotParallelism: 1},
		WritePolicies: map[string]filament.WritePolicy{"times": policy},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sink.Abort(context.Background()) })
	if err := sink.EnsureSchema(ctx, "times", schema); err != nil {
		t.Fatal(err)
	}
	return sink
}

func TestLocalAppendKeepsEndOfDayTimeIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "times.duckdb")
	policy := filament.WritePolicy{
		Capability: filament.WritePolicyCapability{Mode: filament.WriteAppend},
		Resource:   "times",
	}
	sink := openTimesSink(t, ctx, path, "times-run", policy, timesSchema())

	batch := timesBatch(t, timesSchema(), []timesSpec{
		{id: 1, us: 43_200_000_000},
		{id: 2, us: 86_400_000_000},
		{id: 3, us: 86_399_999_999},
		{id: 4, null: true},
	})
	defer batch.Release()
	if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
	// A second, ordinary batch on the same sink exercises the appender path
	// after the statement path on the same connection.
	extra := timesBatch(t, timesSchema(), []timesSpec{
		{id: 5, us: 1_000_000},
		{id: 6, us: 86_399_999_999},
	})
	defer extra.Release()
	if _, err := sink.Apply(ctx, extra, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rows := queryLocal(t, path, `SELECT id, CAST(end_time AS VARCHAR) FROM "main"."times" ORDER BY id`)
	want := [][]any{
		{int64(1), "12:00:00"},
		{int64(2), "24:00:00"},
		{int64(3), "23:59:59.999999"},
		{int64(4), nil},
		{int64(5), "00:00:01"},
		{int64(6), "23:59:59.999999"},
	}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("times = %v, want %v", rows, want)
	}
}

// TestLocalAppendStatementPathRollsBackAndRecoversIntegration checks the
// statement path's all-or-nothing guarantee: a batch whose second row
// duplicates the first row's primary key fails and leaves no rows, and the
// same sink then accepts a corrected batch. The table is pre-created with
// the key because append tables the sink creates carry no primary key, so
// this is the existing destination case.
func TestLocalAppendStatementPathRollsBackAndRecoversIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "times-rollback.duckdb")
	queryLocal(t, path, `CREATE TABLE "main"."times" (
		"id" BIGINT NOT NULL,
		"end_time" TIME,
		PRIMARY KEY ("id")
	)`)
	policy := filament.WritePolicy{
		Capability: filament.WritePolicyCapability{Mode: filament.WriteAppend},
		Resource:   "times",
	}
	sink := openTimesSink(t, ctx, path, "times-rollback-run", policy, timesSchema())

	bad := timesBatch(t, timesSchema(), []timesSpec{
		{id: 1, us: 86_400_000_000},
		{id: 1, us: 43_200_000_000},
	})
	defer bad.Release()
	if _, err := sink.Apply(ctx, bad, filament.ApplyOptions{Policy: policy}); err == nil {
		t.Fatal("Apply with a duplicate primary key succeeded, want an error")
	}
	rows := queryLocal(t, path, `SELECT id FROM "main"."times" ORDER BY id`)
	if len(rows) != 0 {
		t.Fatalf("rows after failed batch = %v, want none", rows)
	}

	good := timesBatch(t, timesSchema(), []timesSpec{
		{id: 1, us: 86_400_000_000},
		{id: 2, us: 43_200_000_000},
	})
	defer good.Release()
	if _, err := sink.Apply(ctx, good, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rows = queryLocal(t, path, `SELECT id, CAST(end_time AS VARCHAR) FROM "main"."times" ORDER BY id`)
	want := [][]any{{int64(1), "24:00:00"}, {int64(2), "12:00:00"}}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("times = %v, want %v", rows, want)
	}
}

// TestLocalAppendStatementPathParsesURNUUIDIntegration covers a UUID in URN
// form on the statement path: the appender path parses it, so the statement
// path must store the same canonical value instead of letting DuckDB reject
// the prefix.
func TestLocalAppendStatementPathParsesURNUUIDIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "urn-uuid.duckdb")
	schema := filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "id", Logical: filament.LogicalInt64},
			{Name: "end_time", Logical: filament.LogicalTime, Nullable: true},
			{Name: "event_id", Logical: filament.LogicalUUID, Nullable: true},
		},
		PrimaryKey: []string{"id"},
	}
	policy := filament.WritePolicy{
		Capability: filament.WritePolicyCapability{Mode: filament.WriteAppend},
		Resource:   "times",
	}
	sink := openTimesSink(t, ctx, path, "urn-uuid-run", policy, schema)

	collector := &integrationCollector{}
	builder := arrowbatch.NewBuilder(arrowbatch.Schema(schema), nil, arrowbatch.Options{MaxRows: 3}, collector)
	builder.Int64(1)
	builder.Time(86_400_000_000)
	builder.String("urn:uuid:" + testUUID(1))
	if err := builder.EndRow(rowmodel.Meta{}); err != nil {
		_ = builder.Close()
		t.Fatal(err)
	}
	if err := builder.Flush(); err != nil {
		_ = builder.Close()
		t.Fatal(err)
	}
	if err := builder.Close(); err != nil {
		t.Fatal(err)
	}
	if len(collector.batches) != 1 {
		t.Fatalf("builder emitted %d batches, want 1", len(collector.batches))
	}
	batch := collector.batches[0]
	batch.Resource = "times"
	defer batch.Release()
	if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got := queryLocal(t, path, `SELECT id, CAST(end_time AS VARCHAR), CAST(event_id AS VARCHAR) FROM "main"."times" ORDER BY id`)
	want := [][]any{{int64(1), "24:00:00", testUUID(1)}}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestLocalReplaceKeepsEndOfDayTimeIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "times-replace.duckdb")
	policy := filament.WritePolicy{
		Capability: filament.WritePolicyCapability{Mode: filament.WriteReplace},
		Resource:   "times",
	}
	sink := openTimesSink(t, ctx, path, "times-replace-run", policy, timesSchema())
	batch := timesBatch(t, timesSchema(), []timesSpec{
		{id: 1, us: 86_400_000_000},
		{id: 2, us: 43_200_000_000},
	})
	defer batch.Release()
	if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	rows := queryLocal(t, path, `SELECT id, CAST(end_time AS VARCHAR) FROM "main"."times" ORDER BY id`)
	want := [][]any{{int64(1), "24:00:00"}, {int64(2), "12:00:00"}}
	if fmt.Sprint(rows) != fmt.Sprint(want) {
		t.Fatalf("times = %v, want %v", rows, want)
	}
}

// TestLocalAppendStatementPathPreservesTypesIntegration covers the three
// values the statement path must carry exactly: a decimal, a logical UUID,
// and a NULL in a destination-only column that has a DEFAULT, which the
// INSERT must not let the default fill.
func TestLocalAppendStatementPathPreservesTypesIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "types.duckdb")
	queryLocal(t, path, `CREATE TABLE "main"."times" (
		"id" BIGINT NOT NULL,
		"end_time" TIME,
		"amount" DECIMAL(18, 2),
		"event_id" UUID,
		"extra" INTEGER DEFAULT 7,
		PRIMARY KEY ("id")
	)`)
	schema := filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "id", Logical: filament.LogicalInt64},
			{Name: "end_time", Logical: filament.LogicalTime, Nullable: true},
			{Name: "amount", Logical: filament.LogicalDecimal, Precision: 18, Scale: 2, Nullable: true},
			{Name: "event_id", Logical: filament.LogicalUUID, Nullable: true},
		},
		PrimaryKey: []string{"id"},
	}
	policy := filament.WritePolicy{
		Capability: filament.WritePolicyCapability{Mode: filament.WriteAppend},
		Resource:   "times",
	}
	sink := openTimesSink(t, ctx, path, "types-run", policy, schema)

	collector := &integrationCollector{}
	builder := arrowbatch.NewBuilder(arrowbatch.Schema(schema), nil, arrowbatch.Options{MaxRows: 3}, collector)
	rows := []struct {
		id  int64
		us  int64
		amt int64
	}{
		{1, 86_400_000_000, 125},
		{2, 43_200_000_000, 250},
	}
	for _, row := range rows {
		builder.Int64(row.id)
		builder.Time(row.us)
		builder.Decimal(decimal128.FromI64(row.amt))
		builder.String(testUUID(row.id))
		if err := builder.EndRow(rowmodel.Meta{}); err != nil {
			_ = builder.Close()
			t.Fatal(err)
		}
	}
	if err := builder.Flush(); err != nil {
		_ = builder.Close()
		t.Fatal(err)
	}
	if err := builder.Close(); err != nil {
		t.Fatal(err)
	}
	if len(collector.batches) != 1 {
		t.Fatalf("builder emitted %d batches, want 1", len(collector.batches))
	}
	batch := collector.batches[0]
	batch.Resource = "times"
	defer batch.Release()
	if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got := queryLocal(t, path, `SELECT id, CAST(end_time AS VARCHAR), CAST(amount AS VARCHAR), CAST(event_id AS VARCHAR), extra FROM "main"."times" ORDER BY id`)
	want := [][]any{
		{int64(1), "24:00:00", "1.25", testUUID(1), nil},
		{int64(2), "12:00:00", "2.50", testUUID(2), nil},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

// TestLocalAppendStatementPathNegativeScaleDecimalIntegration covers the
// decimal edge case CodeRabbit raised on PR #780: arrowbatch.Type permits
// negative scales, and the old decimalText helper panicked slicing past the
// end of the digits for them. A batch with 24:00:00 and a negative-scale
// decimal must reach the statement path and store the exact value: 125 at
// scale -2 is "12500". DuckDB has no negative-scale DECIMAL, so the
// destination column is DECIMAL(18, 0); the value the statement path binds
// is rendered from the Arrow scale, not the column scale.
func TestLocalAppendStatementPathNegativeScaleDecimalIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "negscale.duckdb")
	queryLocal(t, path, `CREATE TABLE "main"."times" (
		"id" BIGINT NOT NULL,
		"end_time" TIME,
		"amount" DECIMAL(18, 0),
		PRIMARY KEY ("id")
	)`)
	schema := filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "id", Logical: filament.LogicalInt64},
			{Name: "end_time", Logical: filament.LogicalTime, Nullable: true},
			{Name: "amount", Logical: filament.LogicalDecimal, Precision: 18, Scale: -2, Nullable: true},
		},
		PrimaryKey: []string{"id"},
	}
	policy := filament.WritePolicy{
		Capability: filament.WritePolicyCapability{Mode: filament.WriteAppend},
		Resource:   "times",
	}
	sink := openTimesSink(t, ctx, path, "negscale-run", policy, schema)

	collector := &integrationCollector{}
	builder := arrowbatch.NewBuilder(arrowbatch.Schema(schema), nil, arrowbatch.Options{MaxRows: 3}, collector)
	rows := []struct {
		id  int64
		us  int64
		amt int64
	}{
		{1, 86_400_000_000, 125},
		{2, 43_200_000_000, -250},
	}
	for _, row := range rows {
		builder.Int64(row.id)
		builder.Time(row.us)
		builder.Decimal(decimal128.FromI64(row.amt))
		if err := builder.EndRow(rowmodel.Meta{}); err != nil {
			_ = builder.Close()
			t.Fatal(err)
		}
	}
	if err := builder.Flush(); err != nil {
		_ = builder.Close()
		t.Fatal(err)
	}
	if err := builder.Close(); err != nil {
		t.Fatal(err)
	}
	if len(collector.batches) != 1 {
		t.Fatalf("builder emitted %d batches, want 1", len(collector.batches))
	}
	batch := collector.batches[0]
	batch.Resource = "times"
	defer batch.Release()
	if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	got := queryLocal(t, path, `SELECT id, CAST(end_time AS VARCHAR), CAST(amount AS VARCHAR) FROM "main"."times" ORDER BY id`)
	want := [][]any{
		{int64(1), "24:00:00", "12500"},
		{int64(2), "12:00:00", "-25000"},
	}
	if fmt.Sprint(got) != fmt.Sprint(want) {
		t.Fatalf("rows = %v, want %v", got, want)
	}
}

func TestLocalParallelApplyIntegration(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "parallel.duckdb")
	schema := integrationSchema()
	policy := writePolicy(filament.WriteAppend, nil)
	sink := openLocalSink(t, ctx, path, "parallel-run", policy, 4, schema)
	const batches, rowsPerBatch = 8, 400
	created := make([]*arrowbatch.Batch, batches)
	for i := range batches {
		created[i] = integrationBatch(t, schema, "events", int64(i*rowsPerBatch+1), rowsPerBatch)
	}
	defer releaseBatches(created)
	applyConcurrently(t, ctx, sink, policy, created)
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	if got, want := queryLocal(t, path, `SELECT count(*) FROM "main"."events"`)[0][0], int64(batches*rowsPerBatch); got != want {
		t.Fatalf("count = %#v, want %d", got, want)
	}
}

func applyConcurrently(t *testing.T, ctx context.Context, sink *Sink, policy filament.WritePolicy, batches []*arrowbatch.Batch) {
	t.Helper()
	var wg sync.WaitGroup
	errs := make(chan error, len(batches))
	for _, batch := range batches {
		wg.Add(1)
		go func() {
			defer wg.Done()
			_, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy})
			errs <- err
		}()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
}

func TestMotherDuckIntegration(t *testing.T) {
	token := os.Getenv("MOTHERDUCK_TOKEN")
	if token == "" {
		t.Skip("MOTHERDUCK_TOKEN is not set")
	}
	ctx := context.Background()
	database := fmt.Sprintf("filament_test_%d", time.Now().UnixNano())
	createMotherDuckDatabase(t, ctx, token, database)
	t.Cleanup(func() { dropMotherDuckDatabase(token, database) })
	schema := integrationSchema()
	policy := writePolicy(filament.WriteAppend, nil)
	sink := New()
	err := sink.Open(ctx, filament.RunSpec{
		Run: "motherduck-run",
		Sink: filament.Ref{Config: map[string]any{
			"token": token, "database": database, "schema": "main",
		}},
		WritePolicies: map[string]filament.WritePolicy{"events": policy},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sink.Abort(context.Background()) })
	if err := sink.EnsureSchema(ctx, "events", schema); err != nil {
		t.Fatal(err)
	}
	applyBatch(t, ctx, sink, integrationBatch(t, schema, "events", 1, 3), policy)
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
}

func createMotherDuckDatabase(t *testing.T, ctx context.Context, token, database string) {
	t.Helper()
	db := openMotherDuck(t, token)
	defer db.Close()
	if _, err := db.ExecContext(ctx, "CREATE DATABASE "+quoteIdent(database)); err != nil {
		t.Fatal(err)
	}
}

func dropMotherDuckDatabase(token, database string) {
	resolved, err := connection.Resolve(filament.NewConfig(map[string]any{"token": token, "database": "my_db"}))
	if err != nil {
		return
	}
	connector, err := connection.Open(resolved)
	if err != nil {
		return
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	_, _ = db.Exec("DROP DATABASE IF EXISTS " + quoteIdent(database))
}

func openMotherDuck(t *testing.T, token string) *sql.DB {
	t.Helper()
	resolved, err := connection.Resolve(filament.NewConfig(map[string]any{"token": token, "database": "my_db"}))
	if err != nil {
		t.Fatal(err)
	}
	connector, err := connection.Open(resolved)
	if err != nil {
		t.Fatal(err)
	}
	return sql.OpenDB(connector)
}

func openLocalSink(
	t *testing.T,
	ctx context.Context,
	path string,
	run filament.RunID,
	policy filament.WritePolicy,
	parallelism int,
	schema filament.RecordSchema,
) *Sink {
	t.Helper()
	sink := newLocal(path)
	err := sink.Open(ctx, filament.RunSpec{
		Run:           run,
		Sink:          filament.Ref{Config: map[string]any{"schema": "main"}},
		Options:       filament.RunOptions{SnapshotParallelism: parallelism},
		WritePolicies: map[string]filament.WritePolicy{"events": policy},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = sink.Abort(context.Background()) })
	if err := sink.EnsureSchema(ctx, "events", schema); err != nil {
		t.Fatal(err)
	}
	return sink
}

// newLocal returns the sink over a local DuckDB file, so the engine runs
// without a MotherDuck token.
func newLocal(path string) *Sink {
	return &Sink{resolve: func(filament.Config) (connection.Resolved, error) {
		return connection.Resolved{Path: path}, nil
	}}
}

func applyBatch(t *testing.T, ctx context.Context, sink *Sink, batch *arrowbatch.Batch, policy filament.WritePolicy) {
	t.Helper()
	defer batch.Release()
	if _, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy}); err != nil {
		t.Fatal(err)
	}
}

func writePolicy(mode filament.WriteMode, keys []string) filament.WritePolicy {
	return filament.WritePolicy{
		Capability: filament.WritePolicyCapability{Mode: mode},
		Resource:   "events",
		Keys:       keys,
	}
}

func integrationSchema() filament.RecordSchema {
	return filament.RecordSchema{
		Fields: []filament.SchemaField{
			{Name: "id", Logical: filament.LogicalInt64},
			{Name: "name", Logical: filament.LogicalString, Nullable: true},
			{Name: "amount", Logical: filament.LogicalDecimal, Precision: 18, Scale: 2, Nullable: true},
			{Name: "created_at", Logical: filament.LogicalTimestamp, Nullable: true},
			{Name: "event_id", Logical: filament.LogicalUUID, Nullable: true},
		},
		PrimaryKey: []string{"id"},
	}
}

type integrationCollector struct {
	batches []*arrowbatch.Batch
}

func (c *integrationCollector) Chunk(batch *arrowbatch.Batch) error {
	c.batches = append(c.batches, batch)
	return nil
}

func (*integrationCollector) Drained(rowmodel.Meta, int) error { return nil }

func integrationBatch(t *testing.T, schema filament.RecordSchema, resource string, start int64, count int) *arrowbatch.Batch {
	t.Helper()
	collector := &integrationCollector{}
	builder := arrowbatch.NewBuilder(arrowbatch.Schema(schema), nil, arrowbatch.Options{MaxRows: count + 1}, collector)
	for offset := range count {
		id := start + int64(offset)
		builder.Int64(id)
		if id%10 == 0 {
			builder.Null()
			builder.Null()
			builder.Null()
			builder.Null()
		} else {
			builder.String(fmt.Sprintf("name-%d", id))
			builder.Decimal(decimal128.FromI64(id*100 + 25))
			builder.Timestamp(1704067200_000000 + id)
			builder.String(testUUID(id))
		}
		if err := builder.EndRow(rowmodel.Meta{}); err != nil {
			_ = builder.Close()
			t.Fatal(err)
		}
	}
	if err := builder.Flush(); err != nil {
		_ = builder.Close()
		t.Fatal(err)
	}
	if err := builder.Close(); err != nil {
		t.Fatal(err)
	}
	if len(collector.batches) != 1 {
		t.Fatalf("builder emitted %d batches, want 1", len(collector.batches))
	}
	collector.batches[0].Resource = resource
	return collector.batches[0]
}

func testUUID(id int64) string {
	return fmt.Sprintf("%08x-0000-4000-8000-%012x", id, id)
}

func releaseBatches(batches []*arrowbatch.Batch) {
	for _, batch := range batches {
		batch.Release()
	}
}

func queryLocal(t *testing.T, path, query string) [][]any {
	t.Helper()
	connector, err := duckdb.NewConnector(path, nil)
	if err != nil {
		t.Fatal(err)
	}
	db := sql.OpenDB(connector)
	defer db.Close()
	result, err := db.Query(query)
	if err != nil {
		t.Fatal(err)
	}
	defer result.Close()
	columns, err := result.Columns()
	if err != nil {
		t.Fatal(err)
	}
	var rows [][]any
	for result.Next() {
		values := make([]any, len(columns))
		targets := make([]any, len(columns))
		for i := range values {
			targets[i] = &values[i]
		}
		if err := result.Scan(targets...); err != nil {
			t.Fatal(err)
		}
		rows = append(rows, values)
	}
	if err := result.Err(); err != nil {
		t.Fatal(err)
	}
	return rows
}

// text renders the driver's decimal and uuid scan types the way SQL would.
func text(v any) string {
	switch value := v.(type) {
	case []byte:
		var id duckdb.UUID
		if err := id.Scan(value); err != nil {
			return string(value)
		}
		return id.String()
	case fmt.Stringer:
		return value.String()
	default:
		return fmt.Sprint(v)
	}
}
