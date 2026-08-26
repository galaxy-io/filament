package arrowbatch

import (
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

func rows(t *testing.T, vals ...int64) *collect {
	t.Helper()
	c := &collect{}
	b := NewBuilder(Schema(testSchema()), nil, Options{MaxRows: 100}, c)
	for _, v := range vals {
		b.Int64(v)
		if v%2 == 0 {
			b.String("x")
		} else {
			b.Null()
		}
		b.Null()
		b.Timestamp(v)
		if err := b.EndRow(rowmodel.Meta{}); err != nil {
			t.Fatal(err)
		}
	}
	if err := b.Flush(); err != nil {
		t.Fatal(err)
	}
	return c
}

func TestCRCDeterministicAndOrderSensitive(t *testing.T) {
	a := rows(t, 1, 2, 3).chunks[0]
	b := rows(t, 1, 2, 3).chunks[0]
	c := rows(t, 3, 2, 1).chunks[0]
	if a.IntegrityCRC() != b.IntegrityCRC() {
		t.Fatal("same rows, different CRC")
	}
	if a.IntegrityCRC() == c.IntegrityCRC() {
		t.Fatal("reordered rows, same CRC")
	}
	if CRC(a.Rows(), Operations{}) == CRC(a.Rows(), NewOperations([]rowmodel.Operation{rowmodel.OpInsert, rowmodel.OpDelete, rowmodel.OpInsert})) {
		t.Fatal("ops ignored")
	}
}
