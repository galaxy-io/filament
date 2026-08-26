package arrowbatch

import (
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

func TestNewOperationsCopiesInput(t *testing.T) {
	input := []rowmodel.Operation{rowmodel.OpInsert, rowmodel.OpUpdate}
	ops := NewOperations(input)
	input[1] = rowmodel.OpDelete
	if got := ops.At(1); got != rowmodel.OpUpdate {
		t.Fatalf("operation changed through constructor input: got %v", got)
	}
	clone := ops.Clone()
	clone[0] = rowmodel.OpDelete
	if got := ops.At(0); got != rowmodel.OpInsert {
		t.Fatalf("operation changed through clone: got %v", got)
	}
}
