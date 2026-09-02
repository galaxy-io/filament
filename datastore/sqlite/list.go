package sqlite

import "fmt"

const connectionColumns = `id, tenant_id, kind, name, connector, config, secret_refs, version, created_at, updated_at, deleted_at, coalesce(created_by_user_id, ''), coalesce(updated_by_user_id, ''), coalesce(deleted_by_user_id, '')`

const pipelineColumns = `id, tenant_id, name, description, current_version_id, worker_configuration, created_at, updated_at, deleted_at, coalesce(created_by_user_id, ''), coalesce(updated_by_user_id, ''), coalesce(deleted_by_user_id, '')`

const pipelineVersionColumns = `id, pipeline_id, version, graph, created_at, updated_at, coalesce(created_by_user_id, ''), coalesce(updated_by_user_id, ''), coalesce(deleted_by_user_id, '')`

// sortClause assembles a list ORDER BY in Go: sqlc's sqlite engine cannot
// bind parameters inside ORDER BY CASE expressions. sortBy is restricted to
// the fixed vocabulary shared with the postgres store; anything else sorts by
// id, so no caller input reaches the SQL.
func sortClause(sortBy string, descending bool) string {
	direction := " ASC"
	if descending {
		direction = " DESC"
	}
	column := "id"
	switch sortBy {
	case "name":
		column = "lower(name)"
	case "version":
		column = "version"
	case "created_at", "updated_at":
		column = sortBy
	}
	if column == "id" {
		return " ORDER BY id" + direction
	}
	return " ORDER BY " + column + direction + ", id" + direction
}

// pageClause renders LIMIT/OFFSET; limit 0 means unlimited.
func pageClause(limit, offset int) string {
	resolved := limit
	if resolved <= 0 {
		resolved = -1
	}
	clause := fmt.Sprintf(" LIMIT %d", resolved)
	if offset > 0 {
		clause += fmt.Sprintf(" OFFSET %d", offset)
	}
	return clause
}
