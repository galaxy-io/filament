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

-- SQLite cannot modify several tables in a CTE. Stream checkpoint saves must
-- run in one transaction using these steps:
--   1. LockReplicationStreamGeneration (or another tenant-scoped stream read).
--   2. GetStreamResourceForCheckpoint; skip retired resources (status = 3).
--   3. DeleteConflictingStreamResourceCheckpoint.
--   4. BumpReplicationStreamMembershipRevision if the resource status is not 2.
--   5. ActivateStreamResourceForCheckpoint, then SaveStreamResourceCheckpoint.
-- Deletion uses the same stream/resource reads, then DeleteStreamResourceCheckpoint.
-- For a non-retired resource, bump the membership revision if its status is not
-- 0, then ResetStreamResourceForCheckpoint. Commit all changes together.

-- name: GetStreamResourceForCheckpoint :one
SELECT r.*
FROM replication_stream_resources AS r
JOIN replication_streams AS s ON s.id = r.replication_stream_id
WHERE s.id = @replication_stream_id
  AND s.tenant_id = @tenant_id
  AND r.resource_name = @resource_name;

-- name: DeleteConflictingStreamResourceCheckpoint :exec
DELETE FROM pipeline_resource_checkpoints
WHERE pipeline_id = @pipeline_id
  AND pipeline_version_id = @pipeline_version_id
  AND route_key = @route_key
  AND resource_name = @resource_name
  AND replication_stream_resource_id IS NOT @resource_id;

-- name: ActivateStreamResourceForCheckpoint :execrows
UPDATE replication_stream_resources
SET status = 2,
    bootstrap_run_id = coalesce(bootstrap_run_id, @last_run_id),
    activated_at = coalesce(activated_at, @activated_at),
    retired_at = NULL,
    error = NULL,
    updated_at = @updated_at
WHERE id = @resource_id AND tenant_id = @tenant_id AND status <> 3;

-- name: SaveStreamResourceCheckpoint :execrows
INSERT INTO pipeline_resource_checkpoints (
  tenant_id, pipeline_id, pipeline_version_id, route_key, resource_name,
  replication_stream_resource_id, cursor, last_run_id, created_at, updated_at
)
SELECT r.tenant_id, @pipeline_id, @pipeline_version_id, @route_key, r.resource_name,
       r.id, @cursor, @last_run_id, @created_at, @updated_at
FROM replication_stream_resources AS r
WHERE r.id = @resource_id AND r.tenant_id = @tenant_id AND r.status = 2
ON CONFLICT (replication_stream_resource_id)
  WHERE replication_stream_resource_id IS NOT NULL
DO UPDATE SET
  tenant_id = excluded.tenant_id,
  pipeline_id = excluded.pipeline_id,
  pipeline_version_id = excluded.pipeline_version_id,
  route_key = excluded.route_key,
  cursor = excluded.cursor,
  last_run_id = excluded.last_run_id,
  updated_at = excluded.updated_at;

-- name: LoadStreamResourceCheckpoint :one
SELECT c.cursor, c.last_run_id, c.updated_at
FROM pipeline_resource_checkpoints AS c
JOIN replication_stream_resources AS r ON r.id = c.replication_stream_resource_id
WHERE r.replication_stream_id = @replication_stream_id
  AND r.resource_name = @resource_name
  AND r.status = 2;

-- name: ListStreamResourceCheckpoints :many
SELECT c.resource_name, c.cursor, c.last_run_id, c.updated_at
FROM pipeline_resource_checkpoints AS c
JOIN replication_stream_resources AS r ON r.id = c.replication_stream_resource_id
WHERE r.replication_stream_id = @replication_stream_id AND r.status = 2
ORDER BY c.resource_name;

-- name: DeleteStreamResourceCheckpoint :exec
DELETE FROM pipeline_resource_checkpoints
WHERE replication_stream_resource_id IN (
  SELECT r.id FROM replication_stream_resources AS r
  JOIN replication_streams AS s ON s.id = r.replication_stream_id
  WHERE s.id = @replication_stream_id AND s.tenant_id = @tenant_id
    AND r.resource_name = @resource_name
);

-- name: ResetStreamResourceForCheckpoint :execrows
UPDATE replication_stream_resources
SET status = 0, activated_at = NULL, error = NULL, updated_at = @updated_at
WHERE id = @resource_id AND tenant_id = @tenant_id AND status <> 3;
