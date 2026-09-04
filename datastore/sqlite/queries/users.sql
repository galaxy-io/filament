-- name: EnsureUser :one
INSERT INTO users (id, tenant_id, external_id, created_at, updated_at)
VALUES (@user_id, @tenant_id, @external_id, @created_at, @updated_at)
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
    updated_at = excluded.updated_at
RETURNING id;
