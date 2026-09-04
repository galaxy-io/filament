-- name: UpsertResource :execrows
INSERT INTO run_resource_states (run_id, resource_name, tenant_id, status, records, bytes, error, created_at, updated_at)
SELECT @run_id, @resource_name, @tenant_id, @status, @records, @bytes, nullif(@error, ''), @created_at, @updated_at
FROM runs WHERE runs.tenant_id = @run_tenant_id AND runs.id = @state_run_id
ON CONFLICT (run_id, resource_name) DO UPDATE SET
    status = excluded.status,
    records = excluded.records,
    bytes = excluded.bytes,
    error = excluded.error,
    updated_at = excluded.updated_at
WHERE run_resource_states.tenant_id = excluded.tenant_id;

-- name: ListResources :many
SELECT run_id, resource_name, tenant_id, status, records, bytes, coalesce(error, '') AS error
FROM run_resource_states WHERE tenant_id = @tenant_id AND run_id = @run_id ORDER BY resource_name;

-- name: ResetRunResources :exec
UPDATE run_resource_states SET
    status = @status,
    records = CASE WHEN cast(@preserve_progress AS boolean) THEN records ELSE 0 END,
    bytes = CASE WHEN cast(@preserve_progress AS boolean) THEN bytes ELSE 0 END,
    error = NULL,
    updated_at = @updated_at
WHERE tenant_id = @tenant_id AND run_id = @run_id;
