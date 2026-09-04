-- name: EnsureTenant :exec
INSERT INTO tenants (id, name, updated_at)
VALUES (@tenant_id, nullif(@name::text, ''), now())
ON CONFLICT (id) DO UPDATE SET
    name = coalesce(EXCLUDED.name, tenants.name),
    updated_at = now();

-- name: ResolveTenant :one
INSERT INTO tenants (external_id, name, updated_at)
VALUES (@external_id::text, nullif(@name::text, ''), now())
ON CONFLICT (external_id) DO UPDATE SET
    name = coalesce(EXCLUDED.name, tenants.name),
    updated_at = now()
RETURNING id;

-- name: LoadTenant :one
SELECT id, coalesce(name, '')::text AS name, created_at, updated_at
FROM tenants
WHERE id = @tenant_id;

-- name: ListTenants :many
SELECT id, coalesce(name, '')::text AS name, created_at, updated_at
FROM tenants
ORDER BY id;
