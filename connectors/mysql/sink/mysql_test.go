package mysql

import (
	"testing"

	"github.com/apache/arrow-go/v18/arrow/decimal128"

	"github.com/galaxy-io/filament/arrowbatch"
	"github.com/galaxy-io/filament/rowmodel"
)

func TestColumnType(t *testing.T) {
	tests := []struct {
		name  string
		field rowmodel.Field
		same  bool
		want  string
	}{
		{"bool", rowmodel.Field{Logical: rowmodel.LogicalBool}, false, "tinyint(1)"},
		{"int16", rowmodel.Field{Logical: rowmodel.LogicalInt16}, false, "smallint"},
		{"bounded decimal", rowmodel.Field{Logical: rowmodel.LogicalDecimal, Precision: 12, Scale: 2}, false, "decimal(12,2)"},
		{"unbounded decimal", rowmodel.Field{Logical: rowmodel.LogicalDecimal}, false, "decimal(65,30)"},
		{"string default", rowmodel.Field{Logical: rowmodel.LogicalString, Native: "character varying(20)"}, false, "longtext"},
		{"same engine keeps native", rowmodel.Field{Logical: rowmodel.LogicalString, Native: "varchar(64)"}, true, "varchar(64)"},
		{"timestamptz avoids 2038 range", rowmodel.Field{Logical: rowmodel.LogicalTimestampTZ}, false, "datetime(6)"},
		{"json", rowmodel.Field{Logical: rowmodel.LogicalJSON}, false, "json"},
		{"array is literal text", rowmodel.Field{Logical: rowmodel.LogicalArray, Native: "text[]"}, false, "longtext"},
		{"uuid", rowmodel.Field{Logical: rowmodel.LogicalUUID}, false, "char(36)"},
		{"unknown", rowmodel.Field{}, false, "longtext"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnType(tt.field, tt.same); got != tt.want {
				t.Fatalf("columnType() = %q, want %q", got, tt.want)
			}
		})
	}
}

type collect struct{ chunks []*arrowbatch.Batch }

func (c *collect) Chunk(ch *arrowbatch.Batch) error { c.chunks = append(c.chunks, ch); return nil }
func (c *collect) Drained(rowmodel.Meta, int) error { return nil }

func TestLoadPayload(t *testing.T) {
	rs := rowmodel.Schema{Fields: []rowmodel.Field{
		{Name: "id", Logical: rowmodel.LogicalInt64},
		{Name: "name", Logical: rowmodel.LogicalString, Nullable: true},
		{Name: "amt", Logical: rowmodel.LogicalDecimal, Precision: 10, Scale: 2},
		{Name: "raw", Logical: rowmodel.LogicalBytes},
		{Name: "at", Logical: rowmodel.LogicalTimestamp},
		{Name: "ok", Logical: rowmodel.LogicalBool},
	}}
	schema := arrowbatch.Schema(rs)
	c := &collect{}
	b := arrowbatch.NewBuilder(schema, nil, arrowbatch.Options{MaxRows: 4}, c)
	b.Int64(1)
	b.String("a\tb\\c\nd")
	b.Decimal(decimal128.FromI64(150))
	b.Bytes([]byte{0, '\t', 0xff})
	b.Timestamp(1704067200_000000)
	b.Bool(true)
	if err := b.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	b.Int64(2)
	b.Null()
	b.Decimal(decimal128.FromI64(-5))
	b.Bytes(nil)
	b.Timestamp(0)
	b.Bool(false)
	if err := b.EndRow(rowmodel.Meta{}); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	defer c.chunks[0].Release()
	rows := c.chunks[0].Rows()
	payload, payloadCRC := newLoader(schema, []int{0, 1, 2, 3, 4, 5}).encode(rows, 0, 2)
	got := string(payload)
	want := "1\ta\\tb\\\\c\\nd\t1.50\t\\0\\t\xff\t2024-01-01 00:00:00\t1\n" +
		"2\t\\N\t-0.05\t\t1970-01-01 00:00:00\t0\n"
	if got != want {
		t.Fatalf("payload\n got %q\nwant %q", got, want)
	}
	if err := verifyLoadChecksum(payload, payloadCRC); err != nil {
		t.Fatalf("payload checksum: %v", err)
	}
	keysPayload, _ := newLoader(schema, []int{0}).encode(rows, 1, 2)
	keys := string(keysPayload)
	if keys != "2\n" {
		t.Fatalf("keys payload = %q", keys)
	}
	sql := loadSQL("filament-1", "`db`.`t`", []string{"`id`", "`name`", "`doc`"}, []bool{false, false, true}, true)
	want = "LOAD DATA LOCAL INFILE 'Reader::filament-1' REPLACE INTO TABLE `db`.`t` CHARACTER SET binary FIELDS TERMINATED BY '\\t' ESCAPED BY '\\\\' LINES TERMINATED BY '\\n' (`id`, `name`, @filament_2) SET `doc` = CONVERT(@filament_2 USING utf8mb4)"
	if sql != want {
		t.Fatalf("load SQL\n got %s\nwant %s", sql, want)
	}
	payload[len(payload)-1] ^= 1
	if err := verifyLoadChecksum(payload, payloadCRC); err == nil {
		t.Fatal("checksum accepted mutated LOAD DATA payload")
	}
}

func TestSpecAdvertisesEncodedIntegrity(t *testing.T) {
	if !New().Spec().Capabilities.EncodedIntegrity {
		t.Fatal("MySQL sink must require encoded integrity evidence")
	}
}
