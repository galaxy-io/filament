-- name: UpsertResource :exec
INSERT INTO run_resource_states (run_id, resource_name, tenant_id, status, records, bytes, error, updated_at)
VALUES (@run_id, @resource_name, @tenant_id, @status, @records, @bytes, nullif(@error::text, ''), now())
ON CONFLICT (run_id, resource_name) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
    status = EXCLUDED.status,
    records = EXCLUDED.records,
    bytes = EXCLUDED.bytes,
    error = EXCLUDED.error,
    updated_at = now();

-- name: ListResources :many
SELECT run_id, resource_name, tenant_id, status, records, bytes, coalesce(error, '')::text AS error
FROM run_resource_states WHERE run_id = @run_id ORDER BY resource_name;

-- name: ResetRunResources :exec
UPDATE run_resource_states SET
    status = @status,
    records = CASE WHEN @preserve_progress::boolean THEN records ELSE 0 END,
    bytes = CASE WHEN @preserve_progress::boolean THEN bytes ELSE 0 END,
    error = NULL,
    updated_at = now()
WHERE run_id = @run_id;
