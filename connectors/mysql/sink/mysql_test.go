package mysql

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/batch"
)

func TestColumnType(t *testing.T) {
	tests := []struct {
		name  string
		field filament.SchemaField
		same  bool
		want  string
	}{
		{"bool", filament.SchemaField{Logical: filament.LogicalBool}, false, "tinyint(1)"},
		{"int16", filament.SchemaField{Logical: filament.LogicalInt16}, false, "smallint"},
		{"bounded decimal", filament.SchemaField{Logical: filament.LogicalDecimal, Precision: 12, Scale: 2}, false, "decimal(12,2)"},
		{"unbounded decimal", filament.SchemaField{Logical: filament.LogicalDecimal}, false, "decimal(65,30)"},
		{"string default", filament.SchemaField{Logical: filament.LogicalString, Native: "character varying(20)"}, false, "longtext"},
		{"same engine keeps native", filament.SchemaField{Logical: filament.LogicalString, Native: "varchar(64)"}, true, "varchar(64)"},
		{"timestamptz avoids 2038 range", filament.SchemaField{Logical: filament.LogicalTimestampTZ}, false, "datetime(6)"},
		{"json", filament.SchemaField{Logical: filament.LogicalJSON}, false, "json"},
		{"array is literal text", filament.SchemaField{Logical: filament.LogicalArray, Native: "text[]"}, false, "longtext"},
		{"uuid", filament.SchemaField{Logical: filament.LogicalUUID}, false, "char(36)"},
		{"unknown", filament.SchemaField{}, false, "longtext"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnType(tt.field, tt.same); got != tt.want {
				t.Fatalf("columnType() = %q, want %q", got, tt.want)
			}
		})
	}
}

type collect struct{ chunks []batch.Chunk }

func (c *collect) Chunk(ch batch.Chunk) error          { c.chunks = append(c.chunks, ch); return nil }
func (c *collect) Drained(filament.RowMeta, int) error { return nil }

func TestLoadPayload(t *testing.T) {
	rs := filament.RecordSchema{Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalInt64},
		{Name: "name", Logical: filament.LogicalString, Nullable: true},
		{Name: "amt", Logical: filament.LogicalDecimal, Precision: 10, Scale: 2},
		{Name: "raw", Logical: filament.LogicalBytes},
		{Name: "at", Logical: filament.LogicalTimestamp},
		{Name: "ok", Logical: filament.LogicalBool},
	}}
	schema := batch.Schema(rs)
	c := &collect{}
	b := batch.New(schema, batch.Options{MaxRows: 4}, c)
	b.Int64(1)
	b.String("a\tb\\c\nd")
	b.Decimal(decimal128.FromI64(150))
	b.Bytes([]byte{0, '\t', 0xff})
	b.Timestamp(1704067200_000000)
	b.Bool(true)
	if err := b.EndRow(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	b.Int64(2)
	b.Null()
	b.Decimal(decimal128.FromI64(-5))
	b.Bytes(nil)
	b.Timestamp(0)
	b.Bool(false)
	if err := b.EndRow(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	rows := c.chunks[0].Rows
	got := string(newLoader(schema, []int{0, 1, 2, 3, 4, 5}).encode(rows, 0, 2))
	want := "1\ta\\tb\\\\c\\nd\t1.50\t\\0\\t\xff\t2024-01-01 00:00:00\t1\n" +
		"2\t\\N\t-0.05\t\t1970-01-01 00:00:00\t0\n"
	if got != want {
		t.Fatalf("payload\n got %q\nwant %q", got, want)
	}
	keys := string(newLoader(schema, []int{0}).encode(rows, 1, 2))
	if keys != "2\n" {
		t.Fatalf("keys payload = %q", keys)
	}
	sql := loadSQL("filament-1", "`db`.`t`", []string{"`id`", "`name`", "`doc`"}, []bool{false, false, true}, true)
	want = "LOAD DATA LOCAL INFILE 'Reader::filament-1' REPLACE INTO TABLE `db`.`t` CHARACTER SET binary FIELDS TERMINATED BY '\\t' ESCAPED BY '\\\\' LINES TERMINATED BY '\\n' (`id`, `name`, @filament_2) SET `doc` = CONVERT(@filament_2 USING utf8mb4)"
	if sql != want {
		t.Fatalf("load SQL\n got %s\nwant %s", sql, want)
	}
}
