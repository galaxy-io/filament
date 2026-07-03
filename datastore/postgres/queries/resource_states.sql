-- name: UpsertResource :exec
INSERT INTO resource_states (run_id, resource_name, tenant_id, enabled, status, records, bytes, error, updated_at)
VALUES (@run_id, @resource_name, @tenant_id, @enabled, @status, @records, @bytes, nullif(@error::text, ''), now())
ON CONFLICT (run_id, resource_name) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
    enabled = EXCLUDED.enabled,
    status = EXCLUDED.status,
    records = EXCLUDED.records,
    bytes = EXCLUDED.bytes,
    error = EXCLUDED.error,
    updated_at = now();

-- name: ListResources :many
SELECT run_id, resource_name, tenant_id, enabled, status, records, bytes, coalesce(error, '')::text AS error
FROM resource_states WHERE run_id = @run_id ORDER BY resource_name;
