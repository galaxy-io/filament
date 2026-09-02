-- SaveResourceCheckpoint inserts directly rather than sourcing from
-- pipelines the way postgres does: the single-writer connection cannot race
-- a pipeline delete, and durable cursors outlive graph rows by design.
-- name: SaveResourceCheckpoint :execrows
INSERT INTO pipeline_resource_checkpoints (
  tenant_id, pipeline_id, pipeline_version_id, route_key, resource_name, cursor, last_run_id, created_at, updated_at
)
VALUES (@tenant_id, @pipeline_id, @pipeline_version_id, @route_key, @resource_name, @cursor, @last_run_id, @created_at, @updated_at)
ON CONFLICT (pipeline_id, pipeline_version_id, route_key, resource_name) DO UPDATE
SET cursor = excluded.cursor, last_run_id = excluded.last_run_id, updated_at = excluded.updated_at
WHERE pipeline_resource_checkpoints.tenant_id = excluded.tenant_id;

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
