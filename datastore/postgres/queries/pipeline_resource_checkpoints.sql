-- name: SaveResourceCheckpoint :execrows
INSERT INTO pipeline_resource_checkpoints (
  tenant_id, pipeline_id, pipeline_version_id, route_key, resource_name, cursor, last_run_id, updated_at
)
SELECT pipelines.tenant_id, @pipeline_id, @pipeline_version_id, @route_key, @resource_name, @cursor, @last_run_id, now()
FROM pipelines
WHERE pipelines.tenant_id = @tenant_id AND pipelines.id = @pipeline_id
ON CONFLICT (pipeline_id, pipeline_version_id, route_key, resource_name) DO UPDATE
SET cursor = EXCLUDED.cursor, last_run_id = EXCLUDED.last_run_id, updated_at = now()
WHERE pipeline_resource_checkpoints.tenant_id = EXCLUDED.tenant_id;

-- name: LoadResourceCheckpoint :one
SELECT cursor, last_run_id, updated_at
FROM pipeline_resource_checkpoints
WHERE tenant_id = @tenant_id
  AND pipeline_id = @pipeline_id
  AND pipeline_version_id = @pipeline_version_id
  AND route_key = @route_key
  AND resource_name = @resource_name;

-- name: ListResourceCheckpoints :many
SELECT resource_name, cursor, last_run_id, updated_at
FROM pipeline_resource_checkpoints
WHERE tenant_id = @tenant_id
  AND pipeline_id = @pipeline_id
  AND pipeline_version_id = @pipeline_version_id
  AND route_key = @route_key
ORDER BY resource_name;

-- name: DeleteResourceCheckpoint :exec
DELETE FROM pipeline_resource_checkpoints
WHERE tenant_id = @tenant_id
  AND pipeline_id = @pipeline_id
  AND pipeline_version_id = @pipeline_version_id
  AND route_key = @route_key
  AND resource_name = @resource_name;
