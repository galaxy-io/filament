package iceberg

import (
	"testing"

	"github.com/apache/iceberg-go/table"
)

func TestCreatePropertiesFollowUpdateMode(t *testing.T) {
	for mode, want := range map[string]string{
		"":                         table.WriteModeMergeOnRead,
		table.WriteModeMergeOnRead: table.WriteModeMergeOnRead,
		table.WriteModeCopyOnWrite: table.WriteModeCopyOnWrite,
	} {
		props := createProperties(mode)
		if props[writeMergeModeKey] != want || props[table.WriteDeleteModeKey] != want {
			t.Fatalf("update mode %q: merge %q, delete %q, want %q", mode, props[writeMergeModeKey], props[table.WriteDeleteModeKey], want)
		}
	}
}
