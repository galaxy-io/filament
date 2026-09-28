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

-- name: SaveStreamResourceCheckpoint :execrows
WITH locked_stream AS MATERIALIZED (
  SELECT s.id
  FROM replication_streams AS s
  WHERE s.id = @replication_stream_id
    AND s.tenant_id = @tenant_id
  FOR UPDATE
), target AS MATERIALIZED (
  SELECT r.id, r.status
  FROM replication_stream_resources AS r
  JOIN locked_stream AS s ON s.id = r.replication_stream_id
  WHERE r.resource_name = @resource_name
    AND r.status <> 3
  FOR UPDATE OF r
), activated AS (
  UPDATE replication_stream_resources AS r
  SET status = 2,
      bootstrap_run_id = COALESCE(r.bootstrap_run_id, @last_run_id),
      activated_at = COALESCE(activated_at, now()),
      retired_at = NULL,
      error = NULL,
      updated_at = now()
  FROM target
  WHERE r.id = target.id
  RETURNING r.id, r.tenant_id, r.replication_stream_id,
            target.status AS previous_status
), bumped_revision AS (
  UPDATE replication_streams AS s
  SET membership_revision = membership_revision + 1
  FROM activated AS a
  WHERE s.id = a.replication_stream_id
    AND a.previous_status <> 2
  RETURNING s.id
), removed_conflict AS (
  DELETE FROM pipeline_resource_checkpoints AS c
  USING activated AS a
  WHERE c.pipeline_id = @pipeline_id
    AND c.pipeline_version_id = @pipeline_version_id
    AND c.route_key = @route_key
    AND c.resource_name = @resource_name
    AND c.replication_stream_resource_id IS DISTINCT FROM a.id
  RETURNING c.id
)
INSERT INTO pipeline_resource_checkpoints (
  tenant_id, pipeline_id, pipeline_version_id, route_key, resource_name,
  replication_stream_resource_id, cursor, last_run_id, updated_at
)
SELECT tenant_id, @pipeline_id, @pipeline_version_id, @route_key, @resource_name,
       id, @cursor, @last_run_id, now()
FROM activated
CROSS JOIN (SELECT count(*) FROM removed_conflict) AS removed
CROSS JOIN (SELECT count(*) FROM bumped_revision) AS bumped
ON CONFLICT (replication_stream_resource_id)
  WHERE replication_stream_resource_id IS NOT NULL
DO UPDATE SET
  tenant_id = EXCLUDED.tenant_id,
  pipeline_id = EXCLUDED.pipeline_id,
  pipeline_version_id = EXCLUDED.pipeline_version_id,
  route_key = EXCLUDED.route_key,
  cursor = EXCLUDED.cursor,
  last_run_id = EXCLUDED.last_run_id,
  updated_at = now();

-- name: LoadStreamResourceCheckpoint :one
SELECT c.cursor, c.last_run_id, c.updated_at
FROM pipeline_resource_checkpoints c
JOIN replication_stream_resources r
  ON r.id = c.replication_stream_resource_id
WHERE r.replication_stream_id = @replication_stream_id
  AND r.resource_name = @resource_name
  AND r.status = 2;

-- name: ListStreamResourceCheckpoints :many
SELECT c.resource_name, c.cursor, c.last_run_id, c.updated_at
FROM pipeline_resource_checkpoints c
JOIN replication_stream_resources r
  ON r.id = c.replication_stream_resource_id
WHERE r.replication_stream_id = @replication_stream_id
  AND r.status = 2
ORDER BY c.resource_name;

-- name: DeleteStreamResourceCheckpoint :exec
WITH locked_stream AS MATERIALIZED (
  SELECT s.id
  FROM replication_streams AS s
  WHERE s.id = @replication_stream_id
    AND s.tenant_id = @tenant_id
  FOR UPDATE
), target AS MATERIALIZED (
  SELECT r.id, r.status
  FROM replication_stream_resources AS r
  JOIN locked_stream AS s ON s.id = r.replication_stream_id
  WHERE r.resource_name = @resource_name
  FOR UPDATE OF r
), deleted AS (
  DELETE FROM pipeline_resource_checkpoints AS c
  USING target
  WHERE c.replication_stream_resource_id = target.id
), reset AS (
  UPDATE replication_stream_resources AS r
  SET status = 0, activated_at = NULL, error = NULL, updated_at = now()
  FROM target
  WHERE r.id = target.id
    AND target.status <> 3
  RETURNING r.replication_stream_id, target.status AS previous_status
)
UPDATE replication_streams AS s
SET membership_revision = membership_revision + 1
FROM reset
WHERE s.id = reset.replication_stream_id
  AND reset.previous_status <> 0;
