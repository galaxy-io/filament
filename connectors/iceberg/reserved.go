package iceberg

import "strings"

// Iceberg metadata table names; catalogs resolve <namespace>.<name> as a
// metadata table lookup, so real tables cannot use them.
var reservedTableNames = map[string]bool{
	"all_data_files":       true,
	"all_delete_files":     true,
	"all_entries":          true,
	"all_files":            true,
	"all_manifests":        true,
	"data_files":           true,
	"delete_files":         true,
	"entries":              true,
	"files":                true,
	"history":              true,
	"manifests":            true,
	"metadata_log_entries": true,
	"partitions":           true,
	"position_deletes":     true,
	"refs":                 true,
	"snapshots":            true,
}

// tableName maps a resource to its destination table name, suffixing "_" when
// the resource collides with a reserved metadata table name.
func tableName(resource string) string {
	if reservedTableNames[strings.ToLower(resource)] {
		return resource + "_"
	}
	return resource
}
