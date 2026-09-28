//go:build integration

package zerobus_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/memory"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/connectors/databricks/zerobus"
	"github.com/galaxy-io/filament/rowmodel"
)

// Integration test for the Databricks Zerobus sink against a real workspace.
// It is gated behind the `integration` build tag and skips unless the workspace
// credentials are supplied via environment variables:
//
//	DBZ_WORKSPACE_URL     https://<workspace>.cloud.databricks.com
//	DBZ_ZEROBUS_ENDPOINT  <workspace-id>.zerobus.<region>.cloud.databricks.com
//	DBZ_CLIENT_ID         service-principal application id
//	DBZ_CLIENT_SECRET     service-principal secret
//	DBZ_CATALOG           Unity Catalog catalog
//	DBZ_SCHEMA            schema
//	DBZ_WAREHOUSE_ID      SQL warehouse id (enables create_table)
//
// With DBZ_WAREHOUSE_ID set the test auto-creates a uniquely named table, so it
// leaves the workspace in a clean, discoverable state. Verify the row count in
// Databricks after the run.
func TestZerobusIngestIntegration(t *testing.T) {
	env := func(k string) string { return os.Getenv(k) }
	for _, k := range []string{"DBZ_WORKSPACE_URL", "DBZ_ZEROBUS_ENDPOINT", "DBZ_CLIENT_ID", "DBZ_CLIENT_SECRET", "DBZ_CATALOG", "DBZ_SCHEMA"} {
		if env(k) == "" {
			t.Skipf("skipping: %s not set", k)
		}
	}

	resource := fmt.Sprintf("filament_zerobus_it_%d", time.Now().Unix())
	cfg := map[string]any{
		"workspace_url":    env("DBZ_WORKSPACE_URL"),
		"zerobus_endpoint": env("DBZ_ZEROBUS_ENDPOINT"),
		"client_id":        env("DBZ_CLIENT_ID"),
		"client_secret":    env("DBZ_CLIENT_SECRET"),
		"catalog":          env("DBZ_CATALOG"),
		"schema":           env("DBZ_SCHEMA"),
	}
	if wh := env("DBZ_WAREHOUSE_ID"); wh != "" {
		cfg["warehouse_id"] = wh
		cfg["create_table"] = true
	} else {
		t.Log("DBZ_WAREHOUSE_ID not set; the target table must already exist")
		if tbl := env("DBZ_TABLE"); tbl != "" {
			resource = tbl
		}
	}

	rs := rowmodel.Schema{
		Resource: resource,
		Fields: []rowmodel.Field{
			{Name: "id", Logical: rowmodel.LogicalInt64, Nullable: true},
			{Name: "name", Logical: rowmodel.LogicalString, Nullable: true},
			{Name: "amount", Logical: rowmodel.LogicalFloat64, Nullable: true},
		},
	}

	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
	defer cancel()

	sink := zerobus.New()
	run := filament.RunSpec{
		Run:           filament.RunID("integration"),
		Sink:          filament.Ref{Connector: "databrickszerobus", Config: cfg},
		WritePolicies: map[string]filament.WritePolicy{"": filament.WritePolicyForIngestion(filament.IngestionFullAppend)},
	}
	if err := sink.TestConnection(ctx, filament.NewConfig(cfg)); err != nil {
		t.Fatalf("TestConnection: %v", err)
	}
	if err := sink.Open(ctx, run); err != nil {
		t.Fatalf("Open: %v", err)
	}
	if err := sink.EnsureSchema(ctx, resource, rs); err != nil {
		_ = sink.Abort(ctx)
		t.Fatalf("EnsureSchema: %v", err)
	}

	batch := buildIntegrationBatch(t, rs)
	defer batch.Release()
	policy := filament.WritePolicyForIngestion(filament.IngestionFullAppend)
	policy.Resource = resource
	receipt, err := sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy})
	if err != nil {
		_ = sink.Abort(ctx)
		t.Fatalf("Apply: %v", err)
	}
	if receipt.Rows != 3 {
		t.Fatalf("receipt.Rows = %d, want 3", receipt.Rows)
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatalf("Commit: %v", err)
	}
	t.Logf("ingested 3 rows into %s.%s.%s", env("DBZ_CATALOG"), env("DBZ_SCHEMA"), resource)
}

// buildIntegrationBatch builds a three-row Arrow batch matching rs for the integration test.
func buildIntegrationBatch(t *testing.T, rs rowmodel.Schema) *arrowbatch.Batch {
	t.Helper()
	schema := arrowbatch.Schema(rs)
	rb := array.NewRecordBuilder(memory.DefaultAllocator, schema)
	defer rb.Release()
	rb.Field(0).(*array.Int64Builder).AppendValues([]int64{1, 2, 3}, nil)
	rb.Field(1).(*array.StringBuilder).AppendValues([]string{"alice", "bob", "carol"}, nil)
	rb.Field(2).(*array.Float64Builder).AppendValues([]float64{10.5, 20.25, 30.0}, nil)
	b := arrowbatch.NewBatch(rb.NewRecord(), arrowbatch.Operations{})
	b.Resource = rs.Resource
	return b
}
