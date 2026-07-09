-- name: CreateConnection :exec
INSERT INTO connections (connection_id, tenant_id, kind, name, provider, config, secret_refs, version, updated_at)
VALUES (@connection_id, @tenant_id, @kind, @name, @provider, @config, @secret_refs, 1, now());

-- name: UpdateConnection :one
UPDATE connections
SET name = @name, provider = @provider, config = @config, secret_refs = @secret_refs,
    version = version + 1, updated_at = now()
WHERE connection_id = @connection_id AND version = @expected_version
RETURNING version;

-- name: GetConnection :one
SELECT connection_id, tenant_id, kind, name, provider, config, secret_refs, version
FROM connections WHERE connection_id = @connection_id;

-- name: ListConnections :many
SELECT connection_id, tenant_id, kind, name, provider, config, secret_refs, version
FROM connections
WHERE (@tenant_id::text = '' OR tenant_id = @tenant_id)
  AND (@kind::smallint = 0 OR kind = @kind)
ORDER BY connection_id;

-- name: DeleteConnection :exec
DELETE FROM connections WHERE connection_id = @connection_id;
