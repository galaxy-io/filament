package runner

import (
	"testing"

	"github.com/galaxy-io/filament/rowmodel"
)

// CDC append keeps every change as history, so the destination drops the
// source key and lets every field go null. Other ingestion keeps the key.
func TestAuditSchemaShapesCDCAppend(t *testing.T) {
	keyed := rowmodel.Schema{Fields: []rowmodel.Field{{Name: "id", Logical: rowmodel.LogicalInt64}}, PrimaryKey: []string{"id"}}
	merge, err := auditSchema(keyed, true, false)
	if err != nil || len(merge.PrimaryKey) != 1 || merge.Fields[0].Nullable {
		t.Fatalf("merge shape: %+v %v", merge, err)
	}
	history, err := auditSchema(keyed, true, true)
	if err != nil || history.PrimaryKey != nil || !history.Fields[0].Nullable || len(history.Fields) != 6 {
		t.Fatalf("history shape: %+v %v", history, err)
	}
}
