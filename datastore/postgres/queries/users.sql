-- name: EnsureUser :one
INSERT INTO users (tenant_id, external_id, updated_at)
VALUES (@tenant_id, @external_id, now())
ON CONFLICT (tenant_id, external_id) DO UPDATE SET
    updated_at = now()
RETURNING id;
