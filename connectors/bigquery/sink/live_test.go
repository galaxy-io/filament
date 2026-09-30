package bigquery

import (
	"context"
	"fmt"
	"os"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/bigquery"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
	"github.com/google/uuid"
)

// Explicit opt-in: this test creates and deletes its own dataset and incurs
// BigQuery usage. Mock tests cannot validate the service's Arrow conversion or SQL.
func TestLiveStorageWrite(t *testing.T) {
	project := os.Getenv("FILAMENT_TEST_BIGQUERY_PROJECT")
	if project == "" {
		t.Skip("set FILAMENT_TEST_BIGQUERY_PROJECT to run the live Storage Write API test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Minute)
	defer cancel()
	dataset := "filament_test_" + strings.ReplaceAll(uuid.NewString(), "-", "")
	client, err := bigquery.NewClient(ctx, project)
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	defer func() {
		cleanup, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		if err := client.Dataset(dataset).DeleteWithContents(cleanup); err != nil {
			t.Errorf("delete test dataset %s: %v", dataset, err)
		}
	}()
	model := rowmodel.Schema{Fields: []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalInt64},
		{Name: "amount", Logical: rowmodel.LogicalDecimal, Precision: 38, Scale: 10},
		{Name: "clock", Logical: rowmodel.LogicalTime},
		{Name: "payload", Logical: rowmodel.LogicalJSON},
		{Name: "civil", Logical: rowmodel.LogicalTimestamp},
		{Name: "instant", Logical: rowmodel.LogicalTimestampTZ},
	}, PrimaryKey: []string{"id"}}
	policy := filament.WritePolicy{Capability: bigQueryWriteCapabilities(filament.IngestionFullUpsert)[0], Keys: []string{"id"}}
	sink := New()
	spec := filament.RunSpec{Run: filament.RunID(uuid.NewString()), Sink: filament.Ref{Config: map[string]any{"project_id": project, "dataset": dataset}}, WritePolicies: map[string]filament.WritePolicy{"records": policy}}
	if err := sink.Open(ctx, spec); err != nil {
		t.Fatal(err)
	}
	defer sink.Abort(context.Background())
	if err := sink.EnsureSchema(ctx, "records", model); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 2; i++ {
		builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(model))
		builder.Field(0).(*array.Int64Builder).Append(-1)
		amount, err := decimal128.FromString("1234567890123456789012345678.1234567890", 38, 10)
		if err != nil {
			t.Fatal(err)
		}
		builder.Field(1).(*array.Decimal128Builder).Append(amount)
		builder.Field(2).(*array.Time64Builder).Append(arrow.Time64(12*3600*1_000_000 + 345678))
		builder.Field(3).(*array.StringBuilder).Append(fmt.Sprintf(`{"version":%d}`, i))
		builder.Field(4).(*array.TimestampBuilder).Append(123456789)
		builder.Field(5).(*array.TimestampBuilder).Append(123456789)
		batch := arrowbatch.NewBatch(builder.NewRecordBatch(), arrowbatch.Operations{})
		builder.Release()
		batch.Resource = "records"
		batch.Seq = uint64(i)
		_, err = sink.Apply(ctx, batch, filament.ApplyOptions{Policy: policy})
		batch.Release()
		if err != nil {
			t.Fatal(err)
		}
	}
	if err := sink.Commit(ctx); err != nil {
		t.Fatal(err)
	}
	query := client.Query("SELECT COUNT(*) AS n, COUNTIF(id = -1 AND amount = BIGNUMERIC '1234567890123456789012345678.1234567890' AND clock = TIME '12:00:00.345678' AND JSON_VALUE(payload, '$.version') = '1' AND civil = DATETIME '1970-01-01 00:02:03.456789' AND instant = TIMESTAMP '1970-01-01 00:02:03.456789+00') AS valid FROM " + qualified(project, dataset, "records"))
	query.Location = sink.location
	result, err := query.Read(ctx)
	if err != nil {
		t.Fatal(err)
	}
	var row struct{ N, Valid int64 }
	if err := result.Next(&row); err != nil {
		t.Fatal(err)
	}
	if row.N != 1 || row.Valid != 1 {
		t.Fatalf("published rows=%d valid=%d; want one final version with preserved types", row.N, row.Valid)
	}
}
