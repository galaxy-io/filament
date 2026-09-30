package bigquery

import (
	"strings"
	"testing"

	"cloud.google.com/go/bigquery"
	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/memory"
	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestStageTypeNormalizationIsLossless(t *testing.T) {
	model := rowmodel.Schema{Fields: []rowmodel.Field{
		{Name: "amount", Logical: rowmodel.LogicalDecimal, Precision: 38, Scale: 10},
		{Name: "time", Logical: rowmodel.LogicalTime},
		{Name: "json", Logical: rowmodel.LogicalJSON},
		{Name: "civil", Logical: rowmodel.LogicalTimestamp},
		{Name: "instant", Logical: rowmodel.LogicalTimestampTZ},
	}}
	definition, err := defineTable("p", "d", "t", model)
	if err != nil {
		t.Fatal(err)
	}
	builder := array.NewRecordBuilder(memory.DefaultAllocator, arrowbatch.Schema(model))
	defer builder.Release()
	amount := "1234567890123456789012345678.1234567890"
	decimal, err := decimal128.FromString(amount, 38, 10)
	if err != nil {
		t.Fatal(err)
	}
	builder.Field(0).(*array.Decimal128Builder).Append(decimal)
	builder.Field(1).(*array.Time64Builder).Append(arrow.Time64(12*3600*1_000_000 + 345678))
	builder.Field(2).(*array.StringBuilder).Append(`{"costOfSaleAccount":{"id":1}}`)
	builder.Field(3).(*array.TimestampBuilder).Append(123456789)
	builder.Field(4).(*array.TimestampBuilder).Append(123456789)
	record := builder.NewRecordBatch()
	batch := arrowbatch.NewBatch(record, arrowbatch.Operations{})
	defer batch.Release()
	staged, err := stageRecord(&tableState{model: model, definition: definition}, batch)
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Release()
	if got := staged.Column(0).(*array.String).Value(0); got != amount {
		t.Fatalf("decimal rounded: %q", got)
	}
	if got := staged.Column(1).(*array.String).Value(0); got != "12:00:00.345678" {
		t.Fatalf("time changed: %q", got)
	}
	for _, i := range []int{2, 3, 4} {
		if staged.Column(i) != batch.Rows().Column(i) {
			t.Fatalf("native field %d unnecessarily copied", i)
		}
	}
	schema := tableSchema(model, true)
	want := []bigquery.FieldType{bigquery.StringFieldType, bigquery.StringFieldType, bigquery.JSONFieldType, bigquery.DateTimeFieldType, bigquery.TimestampFieldType}
	for i, field := range schema {
		if field.Type != want[i] || field.Required {
			t.Fatalf("staging field %d=%+v", i, field)
		}
	}
	sql := insertTableSQL(definition, definition.qualified, "stage")
	if !strings.Contains(sql, "CAST(s.`amount` AS BIGNUMERIC)") || !strings.Contains(sql, "CAST(s.`time` AS TIME)") || strings.Contains(sql, "PARSE_JSON") {
		t.Fatal(sql)
	}
}
func TestDeleteRowsRetainOperationAndGlobalOrder(t *testing.T) {
	model := rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}, {Name: "value", Logical: rowmodel.LogicalString}}, PrimaryKey: []string{"id"}}
	definition, err := defineTable("p", "d", "t", model)
	if err != nil {
		t.Fatal(err)
	}
	original := testBatch("t", 3)
	defer original.Release()
	original.Rows().Retain()
	batch := arrowbatch.NewBatch(original.Rows(), arrowbatch.NewOperations([]rowmodel.Operation{rowmodel.OpUpdate, rowmodel.OpDelete, rowmodel.OpInsert}))
	defer batch.Release()
	staged, err := stageRecord(&tableState{model: model, definition: definition, nextOrdinal: 100}, batch)
	if err != nil {
		t.Fatal(err)
	}
	defer staged.Release()
	ops := staged.Column(2).(*array.Int16)
	ordinals := staged.Column(3).(*array.Int64)
	for i := 0; i < 3; i++ {
		if ops.Value(i) != int16(batch.Op(i)) || ordinals.Value(i) != int64(100+i) {
			t.Fatal("operation or ordinal lost")
		}
	}
	sql := mergeTableSQL(definition, "stage")
	for _, fragment := range []string{"ORDER BY `_filament_internal_ordinal` DESC", "WHEN MATCHED AND s.`_filament_internal_operation` = 2 THEN DELETE", "WHEN NOT MATCHED AND s.`_filament_internal_operation` <> 2 THEN INSERT"} {
		if !strings.Contains(sql, fragment) {
			t.Fatal(sql)
		}
	}
}
