-- name: SaveCheckpoint :execrows
INSERT INTO run_resource_checkpoints (tenant_id, run_id, resource_name, cursor, updated_at)
SELECT runs.tenant_id, @run_id, @resource_name, @cursor, now()
FROM runs
WHERE runs.tenant_id = @tenant_id AND runs.id = @run_id
ON CONFLICT (run_id, resource_name) DO UPDATE SET cursor = EXCLUDED.cursor, updated_at = now()
WHERE run_resource_checkpoints.tenant_id = EXCLUDED.tenant_id;

-- name: LoadCheckpoint :one
SELECT cursor FROM run_resource_checkpoints
WHERE tenant_id = @tenant_id AND run_id = @run_id AND resource_name = @resource_name;
