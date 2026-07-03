-- name: SaveCheckpoint :exec
INSERT INTO checkpoints (run_id, resource_name, cursor, updated_at)
VALUES (@run_id, @resource_name, @cursor, now())
ON CONFLICT (run_id, resource_name) DO UPDATE SET cursor = EXCLUDED.cursor, updated_at = now();

-- name: LoadCheckpoint :one
SELECT cursor FROM checkpoints WHERE run_id = @run_id AND resource_name = @resource_name;
