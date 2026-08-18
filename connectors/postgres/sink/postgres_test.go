package postgres

import (
	"encoding/binary"
	"strings"
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
		{"string", filament.SchemaField{Logical: filament.LogicalString, Native: "varchar(20)"}, false, "text"},
		{"same engine keeps native", filament.SchemaField{Logical: filament.LogicalString, Native: "varchar(20)"}, true, "varchar(20)"},
		{"json", filament.SchemaField{Logical: filament.LogicalJSON, Native: "json"}, false, "jsonb"},
		{"bounded decimal", filament.SchemaField{Logical: filament.LogicalDecimal, Precision: 12, Scale: 2}, false, "numeric(12,2)"},
		{"unbounded decimal", filament.SchemaField{Logical: filament.LogicalDecimal}, false, "numeric"},
		{"unknown", filament.SchemaField{}, false, "text"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := columnType(tt.field, tt.same); got != tt.want {
				t.Fatalf("columnType() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestStatements(t *testing.T) {
	spec := New().Spec()
	found := false
	for _, capability := range spec.Capabilities.WritePolicies {
		if capability.Mode == filament.WriteMerge {
			found = true
		}
	}
	if !found {
		t.Fatal("postgres sink does not advertise CDC merge")
	}
	tbl := &table{
		qualified: `"public"."users"`,
		idents:    []string{`"tenant_id"`, `"id"`, `"name"`},
		types:     []string{"bigint", "uuid", "text"},
		keyIdx:    []int{0, 1},
		temp:      `"_filament_users"`,
		keysTemp:  `"_filament_users_keys"`,
	}
	got := upsertSQL(tbl)
	want := `INSERT INTO "public"."users" ("tenant_id", "id", "name") SELECT DISTINCT ON ("tenant_id", "id") "tenant_id", "id", "name" FROM "_filament_users" ORDER BY "tenant_id", "id", _ord DESC ON CONFLICT ("tenant_id", "id") DO UPDATE SET "name" = excluded."name"`
	if got != want {
		t.Fatalf("upsert\n got %s\nwant %s", got, want)
	}
	del := deleteSQL(tbl.qualified, tbl.keysTemp, []string{`"tenant_id"`, `"id"`})
	for _, part := range []string{`USING "_filament_users_keys" AS k`, `t."tenant_id" = k."tenant_id"`, `t."id" = k."id"`} {
		if !strings.Contains(del, part) {
			t.Fatalf("delete SQL %q does not contain %q", del, part)
		}
	}
	tmp := tempTableSQL(tbl.temp, tbl.idents, tbl.types, true)
	if !strings.HasSuffix(tmp, `"name" text, _ord integer); TRUNCATE "_filament_users"`) {
		t.Fatalf("temp SQL = %s", tmp)
	}
}

// numericWire builds the wire form the tests compare against.
func numericWire(sign uint16, weight int16, dscale uint16, digits ...uint16) []byte {
	b := binary.BigEndian.AppendUint16(nil, uint16(len(digits)))
	b = binary.BigEndian.AppendUint16(b, uint16(weight))
	b = binary.BigEndian.AppendUint16(b, sign)
	b = binary.BigEndian.AppendUint16(b, dscale)
	for _, d := range digits {
		b = binary.BigEndian.AppendUint16(b, d)
	}
	return b
}

func TestNumericWire(t *testing.T) {
	cases := []struct {
		v     decimal128.Num
		scale int
		want  []byte
	}{
		{decimal128.FromI64(150), 2, numericWire(numericPos, 0, 2, 1, 5000)},                     // 1.50
		{decimal128.FromI64(-5), 2, numericWire(numericNeg, -1, 2, 500)},                         // -0.05
		{decimal128.FromI64(0), 2, numericWire(numericPos, 0, 2)},                                // 0.00
		{decimal128.FromI64(123456000), 3, numericWire(numericPos, 1, 3, 12, 3456)},              // 123456.000
		{decimal128.FromI64(100000000), 0, numericWire(numericPos, 2, 0, 1)},                     // 100000000
		{decimal128.FromI64(12345678), 4, numericWire(numericPos, 0, 4, 1234, 5678)},             // 1234.5678
		{decimal128.FromI64(1), 4, numericWire(numericPos, -1, 4, 1)},                            // 0.0001
		{decimal128.New(0, 1<<63), 0, numericWire(numericPos, 4, 0, 922, 3372, 368, 5477, 5808)}, // 2^63
	}
	for _, c := range cases {
		got := appendDecimal(nil, c.v, c.scale)
		if string(got) != string(c.want) {
			t.Errorf("%s scale %d: got %x want %x", c.v.ToString(int32(c.scale)), c.scale, got, c.want)
		}
	}
	texts := map[string][]byte{
		"1.50":                           numericWire(numericPos, 0, 2, 1, 5000),
		"-0.05":                          numericWire(numericNeg, -1, 2, 500),
		"0":                              numericWire(numericPos, 0, 0),
		"NaN":                            numericWire(numericNaN, 0, 0),
		"-Infinity":                      numericWire(numericNInf, 0, 0),
		"12345678901234567890.123456789": numericWire(numericPos, 4, 9, 1234, 5678, 9012, 3456, 7890, 1234, 5678, 9000),
		"0.00001":                        numericWire(numericPos, -2, 5, 1000),
		"00042.10":                       numericWire(numericPos, 0, 2, 42, 1000),
		"123456789012345678901234567890123456789012345678901234567890.5": numericWire(numericPos, 14, 1,
			1234, 5678, 9012, 3456, 7890, 1234, 5678, 9012, 3456, 7890, 1234, 5678, 9012, 3456, 7890, 5000),
	}
	for text, want := range texts {
		got, err := appendNumericText(nil, text)
		if err != nil {
			t.Errorf("%s: %v", text, err)
			continue
		}
		if string(got) != string(want) {
			t.Errorf("%s: got %x want %x", text, got, want)
		}
	}
}

type collect struct{ chunks []batch.Chunk }

func (c *collect) Chunk(ch batch.Chunk) error          { c.chunks = append(c.chunks, ch); return nil }
func (c *collect) Drained(filament.RowMeta, int) error { return nil }

func TestCopierPicksFormat(t *testing.T) {
	rs := filament.RecordSchema{Fields: []filament.SchemaField{
		{Name: "id", Logical: filament.LogicalInt64},
		{Name: "tags", Logical: filament.LogicalArray, Native: "text[]"},
	}}
	schema := batch.Schema(rs)
	c := newCopier(schema, []int{0, 1}, []string{"bigint", "text[]"})
	if c.binary {
		t.Fatal("array column must force text COPY")
	}
	c = newCopier(schema, []int{0}, []string{"bigint"})
	if !c.binary {
		t.Fatal("bigint column has a binary form")
	}
	col := &collect{}
	b := batch.New(schema, batch.Options{MaxRows: 4}, col)
	b.Int64(1)
	b.String("{a,b}")
	if err := b.EndRow(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	b.Int64(2)
	b.Null()
	if err := b.EndRow(filament.RowMeta{}); err != nil {
		t.Fatal(err)
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	rows := col.chunks[0].Rows
	text := newCopier(schema, []int{0, 1}, []string{"bigint", "text[]"}).encode(rows, 0, 2, false)
	if string(text) != "1\t{a,b}\n2\t\\N\n" {
		t.Fatalf("text payload = %q", text)
	}
	bin := newCopier(schema, []int{0}, []string{"bigint"}).encode(rows, 1, 2, false)
	want := append(append([]byte{}, copyHeader...), 0, 1, 0, 0, 0, 8, 0, 0, 0, 0, 0, 0, 0, 2, 0xff, 0xff)
	if string(bin) != string(want) {
		t.Fatalf("binary payload = %x want %x", bin, want)
	}
}
