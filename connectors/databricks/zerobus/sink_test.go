package zerobus

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/ipc"
	"github.com/apache/arrow-go/v18/arrow/memory"
	zb "github.com/databricks/zerobus-sdk/go"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/databricks/internal/connection"
	"github.com/galaxy-io/filament/rowmodel"
)

// --- fakes -------------------------------------------------------------------

// fakeStream is an in-memory ingestStream that records ingested batches.
type fakeStream struct {
	ingested [][]byte
	flushed  int
	closed   int
}

// Ingest records the batch and returns its 1-based offset.
func (f *fakeStream) Ingest(ipcBytes []byte) (int64, error) {
	f.ingested = append(f.ingested, ipcBytes)
	return int64(len(f.ingested)), nil
}

// Flush counts the flush call.
func (f *fakeStream) Flush() error { f.flushed++; return nil }

// Close counts the close call.
func (f *fakeStream) Close() error { f.closed++; return nil }

// fakeClient is an in-memory ingestClient that hands out fakeStreams.
type fakeClient struct {
	streams map[string]*fakeStream
	opened  []string
	closed  int
	openErr error
}

// OpenStream records the table name and returns a fresh fakeStream for it, or
// openErr when set.
func (c *fakeClient) OpenStream(_ context.Context, table string, _ []byte) (ingestStream, error) {
	if c.openErr != nil {
		return nil, c.openErr
	}
	if c.streams == nil {
		c.streams = map[string]*fakeStream{}
	}
	c.opened = append(c.opened, table)
	st := &fakeStream{}
	c.streams[table] = st
	return st, nil
}

// Close counts the close call.
func (c *fakeClient) Close() error { c.closed++; return nil }

// fakeExecutor is an in-memory sqlExecutor that records the statements it is
// asked to run. Statements starting with failPrefix fail.
type fakeExecutor struct {
	statements []string
	warehouses []string
	failPrefix string
}

// Execute records the warehouse and statement without executing anything.
func (f *fakeExecutor) Execute(_ context.Context, warehouse, statement string) error {
	f.warehouses = append(f.warehouses, warehouse)
	f.statements = append(f.statements, statement)
	if f.failPrefix != "" && strings.HasPrefix(statement, f.failPrefix) {
		return errors.New("PERMISSION_DENIED: not the owner")
	}
	return nil
}

// Close is a no-op for the fake.
func (f *fakeExecutor) Close() error { return nil }

// --- helpers -----------------------------------------------------------------

// testSchema returns a small two-column schema used across the sink tests.
func testSchema() rowmodel.Schema {
	return rowmodel.Schema{
		Resource: "users",
		Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt64},
			{Name: "name", Logical: rowmodel.LogicalString, Nullable: true},
		},
	}
}

// testBatch builds a two-row Arrow batch matching rs.
func testBatch(t *testing.T, rs rowmodel.Schema) *arrowbatch.Batch {
	t.Helper()
	schema := arrowbatch.Schema(rs)
	rb := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer rb.Release()
	rb.Field(0).(*array.Int64Builder).AppendValues([]int64{1, 2}, nil)
	rb.Field(1).(*array.StringBuilder).AppendValues([]string{"alice", "bob"}, nil)
	rec := rb.NewRecord()
	b := arrowbatch.NewBatch(rec, arrowbatch.Operations{})
	b.Resource = rs.Resource
	return b
}

// openTestSink builds a Sink wired to the fake client and calls Open with a
// valid config, merging any extra fields.
func openTestSink(t *testing.T, fc *fakeClient, extra map[string]any) *Sink {
	t.Helper()
	s := &Sink{newClient: func(connection.Resolved, string) (ingestClient, error) { return fc, nil }}
	cfg := map[string]any{
		"workspace_url":    "https://dbc-test.cloud.databricks.com",
		"zerobus_endpoint": "1234.zerobus.us-west-2.cloud.databricks.com",
		"client_id":        "app-id",
		"client_secret":    "secret",
		"catalog":          "main",
		"schema":           "sales",
	}
	for k, v := range extra {
		cfg[k] = v
	}
	run := filament.RunSpec{
		Run:           filament.RunID("run-1"),
		Sink:          filament.Ref{Connector: sinkName, Config: cfg},
		WritePolicies: map[string]filament.WritePolicy{"": filament.WritePolicyForIngestion(filament.IngestionFullAppend)},
	}
	if err := s.Open(context.Background(), run); err != nil {
		t.Fatalf("Open: %v", err)
	}
	return s
}

// --- tests -------------------------------------------------------------------

// TestSpecAppendOnly checks the spec advertises schematized, append-only,
// non-streaming capabilities.
func TestSpecAppendOnly(t *testing.T) {
	spec := New().Spec()
	if spec.Name != sinkName {
		t.Fatalf("name = %q, want %q", spec.Name, sinkName)
	}
	if !spec.Capabilities.Schematized {
		t.Fatal("expected Schematized capability")
	}
	if spec.Capabilities.ReusableSession {
		t.Fatal("Zerobus sink must not advertise streaming capabilities")
	}
	if spec.SchemaField != "schema" {
		t.Fatalf("SchemaField = %q, want schema", spec.SchemaField)
	}
	for _, c := range spec.Capabilities.WritePolicies {
		if c.Mode != filament.WriteAppend {
			t.Fatalf("write policy %q is not append-only", c.Mode)
		}
		if c.Durability != filament.DurabilityAfterCommit {
			t.Fatalf("write policy %q durability = %q, want %q (durable only after Commit's Flush)", c.Mode, c.Durability, filament.DurabilityAfterCommit)
		}
	}
}

// TestApplyIngestsBatch checks a batch is ingested and the receipt reports rows.
func TestApplyIngestsBatch(t *testing.T) {
	ctx := context.Background()
	fc := &fakeClient{}
	s := openTestSink(t, fc, nil)
	rs := testSchema()

	if err := s.EnsureSchema(ctx, "users", rs); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if len(fc.opened) != 1 || fc.opened[0] != "main.sales.users" {
		t.Fatalf("opened streams = %v, want [main.sales.users]", fc.opened)
	}

	b := testBatch(t, rs)
	defer b.Release()
	policy := filament.WritePolicyForIngestion(filament.IngestionFullAppend)
	policy.Resource = "users"
	receipt, err := s.Apply(ctx, b, filament.ApplyOptions{Policy: policy})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if receipt.Rows != 2 {
		t.Fatalf("receipt.Rows = %d, want 2", receipt.Rows)
	}
	if receipt.URI != "databricks://main.sales.users" {
		t.Fatalf("receipt.URI = %q", receipt.URI)
	}
	st := fc.streams["main.sales.users"]
	if st == nil || len(st.ingested) != 1 {
		t.Fatalf("expected one ingested batch, got %+v", st)
	}
	// The ingested payload must be a valid Arrow IPC stream of 2 rows.
	r, err := ipc.NewReader(bytes.NewReader(st.ingested[0]))
	if err != nil {
		t.Fatalf("ipc reader: %v", err)
	}
	defer r.Release()
	var rows int64
	for r.Next() {
		rows += r.RecordBatch().NumRows()
	}
	if rows != 2 {
		t.Fatalf("decoded rows = %d, want 2", rows)
	}

	if err := s.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	if st.flushed != 1 || st.closed != 1 {
		t.Fatalf("stream flushed=%d closed=%d, want 1/1", st.flushed, st.closed)
	}
	if fc.closed != 0 {
		t.Fatal("commit closed session")
	}
	if err := s.Close(ctx); err != nil {
		t.Fatal(err)
	}
	if fc.closed != 1 {
		t.Fatalf("client closed=%d, want 1", fc.closed)
	}
}

// TestApplyUnknownResource checks Apply errors when no stream was opened for the resource.
func TestApplyUnknownResource(t *testing.T) {
	ctx := context.Background()
	s := openTestSink(t, &fakeClient{}, nil)
	b := testBatch(t, testSchema())
	defer b.Release()
	policy := filament.WritePolicyForIngestion(filament.IngestionFullAppend)
	_, err := s.Apply(ctx, b, filament.ApplyOptions{Policy: policy})
	if err == nil || !strings.Contains(err.Error(), "no schema ensured") {
		t.Fatalf("expected no-schema-ensured error, got %v", err)
	}
}

// openCreateSink opens a sink with create_table on, wired to the given fakes.
func openCreateSink(t *testing.T, fc *fakeClient, fe *fakeExecutor) *Sink {
	t.Helper()
	s := openTestSink(t, fc, map[string]any{"create_table": true, "warehouse_id": "wh-123"})
	s.newExecutor = func(connection.Resolved) (sqlExecutor, error) { return fe, nil }
	return s
}

// TestEnsureSchemaCreatesSchemaTablesAndGrants checks create_table creates the
// schema and grants USE SCHEMA once per run, then creates and grants each table,
// all before the stream opens.
func TestEnsureSchemaCreatesSchemaTablesAndGrants(t *testing.T) {
	ctx := context.Background()
	fc := &fakeClient{}
	fe := &fakeExecutor{}
	s := openCreateSink(t, fc, fe)
	for _, resource := range []string{"users", "orders"} {
		rs := testSchema()
		rs.Resource = resource
		if err := s.EnsureSchema(ctx, resource, rs); err != nil {
			t.Fatalf("EnsureSchema(%s): %v", resource, err)
		}
	}
	want := []string{
		"CREATE SCHEMA IF NOT EXISTS `main`.`sales`",
		"GRANT USE SCHEMA ON SCHEMA `main`.`sales` TO `app-id`",
		"CREATE TABLE IF NOT EXISTS `main`.`sales`.`users`",
		"GRANT SELECT, MODIFY ON TABLE `main`.`sales`.`users` TO `app-id`",
		"CREATE TABLE IF NOT EXISTS `main`.`sales`.`orders`",
		"GRANT SELECT, MODIFY ON TABLE `main`.`sales`.`orders` TO `app-id`",
	}
	if len(fe.statements) != len(want) {
		t.Fatalf("statements = %q, want %d", fe.statements, len(want))
	}
	for i, w := range want {
		if !strings.HasPrefix(fe.statements[i], w) {
			t.Errorf("statement %d = %q, want prefix %q", i, fe.statements[i], w)
		}
	}
	for _, wh := range fe.warehouses {
		if wh != "wh-123" {
			t.Fatalf("warehouses = %v", fe.warehouses)
		}
	}
	if len(fc.opened) != 2 {
		t.Fatalf("opened streams = %v", fc.opened)
	}
}

// TestCreateTableGrantFailureIsNotFatal checks a failed grant does not stop the
// run, since the grants may already be in place on an object someone else owns.
func TestCreateTableGrantFailureIsNotFatal(t *testing.T) {
	fc := &fakeClient{}
	s := openCreateSink(t, fc, &fakeExecutor{failPrefix: "GRANT"})
	if err := s.EnsureSchema(context.Background(), "users", testSchema()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if len(fc.opened) != 1 {
		t.Fatalf("opened streams = %v, want the stream opened", fc.opened)
	}
}

// TestStreamFailureReportsFailedGrants checks a stream that fails to open
// carries the grants that failed before it, the likely cause.
func TestStreamFailureReportsFailedGrants(t *testing.T) {
	fc := &fakeClient{openErr: errors.New("401 invalid_authorization_details")}
	s := openCreateSink(t, fc, &fakeExecutor{failPrefix: "GRANT"})
	err := s.EnsureSchema(context.Background(), "users", testSchema())
	if err == nil {
		t.Fatal("expected open stream error")
	}
	for _, want := range []string{"invalid_authorization_details", "grant USE SCHEMA", "grant SELECT, MODIFY on main.sales.users", "PERMISSION_DENIED"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q does not contain %q", err, want)
		}
	}
}

// TestOpenResetsSchemaSetup checks a reopened sink creates and grants the
// schema again for the new run.
func TestOpenResetsSchemaSetup(t *testing.T) {
	ctx := context.Background()
	fe := &fakeExecutor{failPrefix: "GRANT"}
	s := openCreateSink(t, &fakeClient{}, fe)
	if err := s.EnsureSchema(ctx, "users", testSchema()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if err := s.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	fe.statements = nil
	fe.failPrefix = ""
	run := filament.RunSpec{
		Run: filament.RunID("run-2"),
		Sink: filament.Ref{Connector: sinkName, Config: map[string]any{
			"workspace_url": "https://dbc-test.cloud.databricks.com", "zerobus_endpoint": "1234.zerobus.us-west-2.cloud.databricks.com",
			"client_id": "app-id", "client_secret": "secret", "catalog": "main", "schema": "sales",
			"create_table": true, "warehouse_id": "wh-123",
		}},
		WritePolicies: map[string]filament.WritePolicy{"": filament.WritePolicyForIngestion(filament.IngestionFullAppend)},
	}
	if err := s.Open(ctx, run); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if len(s.setupErrs) != 0 {
		t.Fatalf("setupErrs carried across runs: %v", s.setupErrs)
	}
	if err := s.EnsureSchema(ctx, "users", testSchema()); err != nil {
		t.Fatalf("EnsureSchema: %v", err)
	}
	if len(fe.statements) == 0 || !strings.HasPrefix(fe.statements[0], "CREATE SCHEMA") {
		t.Fatalf("statements = %q, want CREATE SCHEMA first", fe.statements)
	}
}

// TestCreateTableRequiresWarehouse checks create_table without a warehouse_id errors.
func TestCreateTableRequiresWarehouse(t *testing.T) {
	ctx := context.Background()
	s := openTestSink(t, &fakeClient{}, map[string]any{"create_table": true})
	s.newExecutor = func(connection.Resolved) (sqlExecutor, error) { return &fakeExecutor{}, nil }
	if err := s.EnsureSchema(ctx, "users", testSchema()); err == nil {
		t.Fatal("expected create_table without warehouse_id to fail")
	}
}

// TestCreateTableDDL checks the rendered DDL, including quoting and lineage columns.
func TestCreateTableDDL(t *testing.T) {
	ddl, err := createTableDDL("main", "sales", "users", testSchema())
	if err != nil {
		t.Fatalf("createTableDDL: %v", err)
	}
	want := "CREATE TABLE IF NOT EXISTS `main`.`sales`.`users` (`id` BIGINT NOT NULL, `name` STRING) USING DELTA"
	if ddl != want {
		t.Fatalf("ddl=\n%s\nwant\n%s", ddl, want)
	}
}

// TestDeltaTypeTimeUnsupported checks the time logical type is rejected on the DDL path.
func TestDeltaTypeTimeUnsupported(t *testing.T) {
	if _, err := deltaType(rowmodel.Field{Name: "t", Logical: rowmodel.LogicalTime}); err == nil {
		t.Fatal("expected time logical type to be unsupported for auto-create")
	}
}

// TestValidateRequiresFields checks Validate rejects missing fields and accepts a full config.
func TestValidateRequiresFields(t *testing.T) {
	s := New()
	if err := s.Validate(filament.NewConfig(map[string]any{"workspace_url": "https://x"})); err == nil {
		t.Fatal("expected validation error for missing fields")
	}
	full := filament.NewConfig(map[string]any{
		"workspace_url": "https://x", "zerobus_endpoint": "y", "client_id": "z", "client_secret": "s", "catalog": "c",
	})
	if err := s.Validate(full); err != nil {
		t.Fatalf("Validate: %v", err)
	}
}

// TestValidateRejectsUnknownCompression checks an unknown ipc_compression value is rejected.
func TestValidateRejectsUnknownCompression(t *testing.T) {
	s := New()
	cfg := filament.NewConfig(map[string]any{
		"workspace_url": "https://x", "zerobus_endpoint": "y", "client_id": "z", "client_secret": "s", "catalog": "c",
		"ipc_compression": "zstandard",
	})
	if err := s.Validate(cfg); err == nil {
		t.Fatal("expected error for unknown ipc_compression")
	}
}

// TestCompressionFor checks each ipc_compression value maps to the expected SDK codec.
func TestCompressionFor(t *testing.T) {
	cases := map[string]zb.IPCCompressionType{
		connection.CompressionNone: zb.IPCCompressionNone,
		connection.CompressionLZ4:  zb.IPCCompressionLZ4Frame,
		connection.CompressionZstd: zb.IPCCompressionZstd,
		"":                         zb.IPCCompressionNone,
		"bogus":                    zb.IPCCompressionNone,
	}
	for in, want := range cases {
		if got := compressionFor(in); got != want {
			t.Errorf("compressionFor(%q) = %v, want %v", in, got, want)
		}
	}
}
