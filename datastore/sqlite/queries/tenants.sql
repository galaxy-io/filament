-- name: EnsureTenant :exec
INSERT INTO tenants (id, name, created_at, updated_at)
VALUES (@tenant_id, nullif(@name, ''), @created_at, @updated_at)
ON CONFLICT (id) DO UPDATE SET
    name = coalesce(excluded.name, tenants.name),
    updated_at = excluded.updated_at;

-- name: ResolveTenant :one
INSERT INTO tenants (id, external_id, name, created_at, updated_at)
VALUES (@tenant_id, @external_id, nullif(@name, ''), @created_at, @updated_at)
ON CONFLICT (external_id) DO UPDATE SET
    name = coalesce(excluded.name, tenants.name),
    updated_at = excluded.updated_at
RETURNING id;

-- name: LoadTenant :one
SELECT id, coalesce(name, '') AS name, created_at, updated_at
FROM tenants
WHERE id = @tenant_id;

-- name: ListTenants :many
SELECT id, coalesce(name, '') AS name, created_at, updated_at
FROM tenants
ORDER BY id;
