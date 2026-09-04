-- name: CreateConnection :exec
INSERT INTO connections (id, tenant_id, kind, name, connector, config, secret_refs, version, updated_at)
VALUES (@connection_id, @tenant_id, @kind, @name, @connector, @config, @secret_refs, 1, now());

-- name: UpdateConnection :one
UPDATE connections
SET name = @name, connector = @connector, config = @config, secret_refs = @secret_refs,
    version = version + 1, updated_at = now()
WHERE tenant_id = @tenant_id AND id = @connection_id AND version = @expected_version AND NOT is_deleted
RETURNING version;

-- name: GetConnection :one
SELECT id, tenant_id, kind, name, connector, config, secret_refs, version, created_at, updated_at, deleted_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id
FROM connections WHERE tenant_id = @tenant_id AND id = @connection_id;

-- name: ListConnections :many
SELECT id, tenant_id, kind, name, connector, config, secret_refs, version, created_at, updated_at, deleted_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id
FROM connections
WHERE tenant_id = @tenant_id
  AND (sqlc.narg('kind')::connector_kind IS NULL OR kind = sqlc.narg('kind'))
  AND (@include_deleted::boolean OR NOT is_deleted)
  AND (nullif(@search::text, '') IS NULL
       OR name ILIKE '%' || @search || '%' ESCAPE '\'
       OR connector ILIKE '%' || @search || '%' ESCAPE '\')
ORDER BY
  CASE WHEN @sort_by::text IN ('', 'id') AND NOT @sort_desc::boolean THEN id END ASC,
  CASE WHEN @sort_by::text IN ('', 'id') AND @sort_desc::boolean THEN id END DESC,
  CASE WHEN @sort_by::text = 'name' AND NOT @sort_desc::boolean THEN lower(name) END ASC,
  CASE WHEN @sort_by::text = 'name' AND @sort_desc::boolean THEN lower(name) END DESC,
  CASE WHEN @sort_by::text = 'created_at' AND NOT @sort_desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_by::text = 'created_at' AND @sort_desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_by::text = 'updated_at' AND NOT @sort_desc::boolean THEN updated_at END ASC,
  CASE WHEN @sort_by::text = 'updated_at' AND @sort_desc::boolean THEN updated_at END DESC,
  CASE WHEN NOT @sort_desc::boolean THEN id END ASC,
  CASE WHEN @sort_desc::boolean THEN id END DESC
LIMIT NULLIF(@lim::int, 0)
OFFSET @offset_rows::int;

-- name: CountConnections :one
SELECT count(*) FROM connections
WHERE tenant_id = @tenant_id
  AND (sqlc.narg('kind')::connector_kind IS NULL OR kind = sqlc.narg('kind'))
  AND (@include_deleted::boolean OR NOT is_deleted)
  AND (nullif(@search::text, '') IS NULL
       OR name ILIKE '%' || @search || '%' ESCAPE '\'
       OR connector ILIKE '%' || @search || '%' ESCAPE '\');

-- name: DeleteConnection :exec
UPDATE connections
SET
  name = name || '__deleted__' || to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
  is_deleted = true,
  deleted_at = now(),
  updated_at = now()
WHERE tenant_id = @tenant_id AND id = @connection_id AND NOT is_deleted;
