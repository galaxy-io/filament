-- name: SaveResourceCheckpoint :exec
INSERT INTO pipeline_resource_checkpoints (
  id, pipeline_id, pipeline_version_id, route_key, resource_name, cursor, last_run_id, updated_at
) VALUES (
  jsonb_build_array(@pipeline_id::text, @pipeline_version_id::text, @route_key::text, @resource_name::text)::text,
  @pipeline_id, @pipeline_version_id, @route_key, @resource_name, @cursor, @last_run_id, now()
)
ON CONFLICT (pipeline_id, pipeline_version_id, route_key, resource_name) DO UPDATE
SET cursor = EXCLUDED.cursor, last_run_id = EXCLUDED.last_run_id, updated_at = now();

-- name: LoadResourceCheckpoint :one
SELECT cursor, last_run_id, updated_at
FROM pipeline_resource_checkpoints
WHERE pipeline_id = @pipeline_id
  AND pipeline_version_id = @pipeline_version_id
  AND route_key = @route_key
  AND resource_name = @resource_name;

-- name: DeleteResourceCheckpoint :exec
DELETE FROM pipeline_resource_checkpoints
WHERE pipeline_id = @pipeline_id
  AND pipeline_version_id = @pipeline_version_id
  AND route_key = @route_key
  AND resource_name = @resource_name;
