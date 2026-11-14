package postgres

import (
	"strings"
	"testing"

	"github.com/apache/arrow-go/v18/arrow"
	"github.com/apache/arrow-go/v18/arrow/array"
	"github.com/apache/arrow-go/v18/arrow/decimal128"
	"github.com/apache/arrow-go/v18/arrow/memory"
)

func TestCopierDecimalText(t *testing.T) {
	tests := []struct {
		name  string
		scale int32
		want  string
		zero  string
	}{
		{"ordinary negative scale", -2, "1234500", "0"},
		{"fixed-point boundary", 38, "0." + strings.Repeat("0", 33) + "12345", "0." + strings.Repeat("0", 38)},
		{"positive exponent boundary", -39, "12345e39", "0"},
		{"negative exponent boundary", 39, "12345e-39", "0"},
		{"minimum Postgres scale", -1000, "12345e1000", "0"},
		{"maximum Postgres scale", 1000, "12345e-1000", "0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			schema := arrow.NewSchema([]arrow.Field{
				{Name: "amount", Type: &arrow.Decimal128Type{Precision: 38, Scale: tt.scale}, Nullable: true},
				{Name: "tags", Type: arrow.BinaryTypes.String},
			}, nil)
			builder := array.NewRecordBuilder(memory.DefaultAllocator, schema)
			defer builder.Release()
			amount := builder.Field(0).(*array.Decimal128Builder)
			amount.Append(decimal128.FromI64(12345))
			amount.Append(decimal128.FromI64(-12345))
			amount.Append(decimal128.FromI64(0))
			amount.AppendNull()
			tags := builder.Field(1).(*array.StringBuilder)
			for range 4 {
				tags.Append("{}")
			}
			rows := builder.NewRecordBatch()
			defer rows.Release()

			// An array destination makes COPY use the decimal text renderer.
			copier := newCopier(schema, []int{0, 1}, []string{"numeric", "text[]"})
			if copier.binary {
				t.Fatal("array column must force text COPY")
			}
			got, checksum := copier.encode(rows, 0, 4, false)
			want := tt.want + "\t{}\n-" + tt.want + "\t{}\n" + tt.zero + "\t{}\n\\N\t{}\n"
			if string(got) != want {
				t.Fatalf("COPY payload = %q, want %q", got, want)
			}
			if err := verifyCopyChecksum(got, checksum); err != nil {
				t.Fatal(err)
			}
		})
	}
}
