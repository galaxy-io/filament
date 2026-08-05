-- name: SaveResourceCheckpoint :exec
INSERT INTO resource_checkpoints (
  pipeline_id, pipeline_version, route_key, resource_name, cursor, last_run_id, updated_at
) VALUES (
  @pipeline_id, @pipeline_version, @route_key, @resource_name, @cursor, @last_run_id, now()
)
ON CONFLICT (pipeline_id, pipeline_version, route_key, resource_name) DO UPDATE
SET cursor = EXCLUDED.cursor, last_run_id = EXCLUDED.last_run_id, updated_at = now();

-- name: LoadResourceCheckpoint :one
SELECT cursor, last_run_id, updated_at
FROM resource_checkpoints
WHERE pipeline_id = @pipeline_id
  AND pipeline_version = @pipeline_version
  AND route_key = @route_key
  AND resource_name = @resource_name;

-- name: DeleteResourceCheckpoint :exec
DELETE FROM resource_checkpoints
WHERE pipeline_id = @pipeline_id
  AND pipeline_version = @pipeline_version
  AND route_key = @route_key
  AND resource_name = @resource_name;
