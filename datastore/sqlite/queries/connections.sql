-- name: CreateConnection :exec
INSERT INTO connections (id, tenant_id, kind, name, connector, config, secret_refs, version, created_at, updated_at)
VALUES (@connection_id, @tenant_id, @kind, @name, @connector, @config, @secret_refs, 1, @created_at, @updated_at);

-- name: UpdateConnection :one
UPDATE connections
SET name = @name, connector = @connector, config = @config, secret_refs = @secret_refs,
    version = version + 1, updated_at = @updated_at
WHERE tenant_id = @tenant_id AND id = @connection_id AND version = @expected_version AND is_deleted = 0
RETURNING version;

-- name: GetConnection :one
SELECT id, tenant_id, kind, name, connector, config, secret_refs, version, created_at, updated_at, deleted_at,
       coalesce(created_by_user_id, '') AS created_by_user_id,
       coalesce(updated_by_user_id, '') AS updated_by_user_id,
       coalesce(deleted_by_user_id, '') AS deleted_by_user_id
FROM connections WHERE tenant_id = @tenant_id AND id = @connection_id;

-- name: CountConnections :one
SELECT count(*) FROM connections
WHERE tenant_id = @tenant_id
  AND (cast(@kind AS text) = '' OR kind = @kind_filter)
  AND (cast(@include_deleted AS boolean) OR is_deleted = 0)
  AND (cast(@search AS text) = ''
       OR instr(lower(name), lower(@name_search)) > 0
       OR instr(lower(connector), lower(@connector_search)) > 0);

-- name: DeleteConnection :exec
UPDATE connections
SET name = name || @delete_stamp, is_deleted = 1, deleted_at = @deleted_at, updated_at = @updated_at
WHERE tenant_id = @tenant_id AND id = @connection_id AND is_deleted = 0;
