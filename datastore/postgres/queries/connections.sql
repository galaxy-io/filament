-- name: CreateConnection :exec
INSERT INTO connections (connection_id, tenant_id, kind, name, provider, config, secret_refs, version, updated_at)
VALUES (@connection_id, @tenant_id, @kind, @name, @provider, @config, @secret_refs, 1, now());

-- name: UpdateConnection :one
UPDATE connections
SET name = @name, provider = @provider, config = @config, secret_refs = @secret_refs,
    version = version + 1, updated_at = now()
WHERE connection_id = @connection_id AND version = @expected_version AND NOT is_deleted
RETURNING version;

-- name: GetConnection :one
SELECT connection_id, tenant_id, kind, name, provider, config, secret_refs, version
FROM connections WHERE connection_id = @connection_id AND NOT is_deleted;

-- name: ListConnections :many
SELECT connection_id, tenant_id, kind, name, provider, config, secret_refs, version
FROM connections
WHERE NOT is_deleted
  AND (@tenant_id::text = '' OR tenant_id = @tenant_id)
  AND (sqlc.narg('kind')::connection_kind IS NULL OR kind = sqlc.narg('kind'))
ORDER BY connection_id;

-- name: DeleteConnection :exec
UPDATE connections
SET
  name = name || '_deleted_' || extract(epoch from CURRENT_TIMESTAMP)::bigint::text,
  is_deleted = true,
  updated_at = CURRENT_TIMESTAMP
WHERE connection_id = @connection_id AND NOT is_deleted;
