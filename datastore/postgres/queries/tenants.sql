-- name: EnsureTenant :exec
INSERT INTO tenants (tenant_id, name, updated_at)
VALUES (@tenant_id, nullif(@name::text, ''), now())
ON CONFLICT (tenant_id) DO UPDATE SET
    name = coalesce(EXCLUDED.name, tenants.name),
    updated_at = now();

-- name: LoadTenant :one
SELECT tenant_id, coalesce(name, '')::text AS name, created_at, updated_at
FROM tenants
WHERE tenant_id = @tenant_id;

-- name: ListTenants :many
SELECT tenant_id, coalesce(name, '')::text AS name, created_at, updated_at
FROM tenants
ORDER BY tenant_id;
