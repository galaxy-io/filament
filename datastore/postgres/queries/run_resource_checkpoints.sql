-- name: SaveCheckpoint :exec
INSERT INTO run_resource_checkpoints (tenant_id, run_id, resource_name, cursor, updated_at)
SELECT tenant_id, @run_id, @resource_name, @cursor, now()
FROM runs
WHERE id = @run_id
ON CONFLICT (run_id, resource_name) DO UPDATE SET cursor = EXCLUDED.cursor, updated_at = now();

-- name: LoadCheckpoint :one
SELECT cursor FROM run_resource_checkpoints WHERE run_id = @run_id AND resource_name = @resource_name;
