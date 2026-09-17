-- Lock queries must run in the same transaction as their dependent writes.
-- SQLite's single-writer connection supplies serialization instead of FOR UPDATE.
-- IDs are supplied by Go; lifecycle stamps use unix milliseconds.

-- name: LockPipelineForReplicationStream :one
SELECT id
FROM pipelines
WHERE id = @pipeline_id
  AND tenant_id = @tenant_id
  AND deleted_at IS NULL;

-- name: GetActiveReplicationStream :one
SELECT *
FROM replication_streams
WHERE pipeline_id = @pipeline_id
  AND route_key = @route_key
  AND status = 0;

-- name: GetReplicationStream :one
SELECT *
FROM replication_streams
WHERE id = @replication_stream_id;

-- name: ListRetiredReplicationStreamsForRoute :many
SELECT *
FROM replication_streams
WHERE pipeline_id = @pipeline_id
  AND route_key = @route_key
  AND status = 1
  AND coalesce(json_type(consumer_config, '$._filament_cleanup_complete'), '') <> 'true'
ORDER BY generation;

-- name: MarkReplicationStreamCleaned :exec
UPDATE replication_streams
SET consumer_config = json_set(consumer_config, '$._filament_cleanup_complete', json('true')),
    updated_at = @updated_at
WHERE id = @replication_stream_id
  AND status = 1;

-- name: NextReplicationStreamGeneration :one
SELECT CAST(COALESCE(MAX(generation), 0) + 1 AS INTEGER)
FROM replication_streams
WHERE pipeline_id = @pipeline_id
  AND route_key = @route_key;

-- name: RetireReplicationStream :exec
UPDATE replication_streams
SET status = 1, retired_at = @retired_at, error = NULL, updated_at = @updated_at
WHERE id = @replication_stream_id
  AND status = 0;

-- name: CreateReplicationStream :one
INSERT INTO replication_streams (
  id, tenant_id, pipeline_id, route_key, generation,
  source_connection_id, sink_connection_id,
  consumer_name, consumer_config, continuity_fingerprint,
  status, created_from_pipeline_version_id, created_at, updated_at
) VALUES (
  @replication_stream_id, @tenant_id, @pipeline_id, @route_key, @generation,
  @source_connection_id, @sink_connection_id,
  @consumer_name, @consumer_config, @continuity_fingerprint,
  0, @created_from_pipeline_version_id, @created_at, @updated_at
)
RETURNING *;

-- name: LockReplicationStream :one
SELECT id
FROM replication_streams
WHERE id = @replication_stream_id
  AND tenant_id = @tenant_id
  AND status = 0;

-- name: BumpReplicationStreamMembershipRevision :exec
UPDATE replication_streams
SET membership_revision = membership_revision + 1
WHERE id = @replication_stream_id
  AND tenant_id = @tenant_id;

-- name: UpsertReplicationStreamResource :exec
INSERT INTO replication_stream_resources (
  id, replication_stream_id, tenant_id, resource_name, status,
  bootstrap_mode, bootstrap_config, created_at, updated_at
) VALUES (
  @resource_id, @replication_stream_id, @tenant_id, @resource_name, 0,
  @bootstrap_mode, @bootstrap_config, @created_at, @updated_at
)
ON CONFLICT (replication_stream_id, resource_name) DO UPDATE
SET status = CASE
      WHEN replication_stream_resources.status = 3 THEN 0
      ELSE replication_stream_resources.status
    END,
    bootstrap_mode = EXCLUDED.bootstrap_mode,
    bootstrap_config = EXCLUDED.bootstrap_config,
    bootstrap_run_id = CASE
      WHEN replication_stream_resources.status = 3 THEN NULL
      ELSE replication_stream_resources.bootstrap_run_id
    END,
    bootstrap_started_at = CASE
      WHEN replication_stream_resources.status = 3 THEN NULL
      ELSE replication_stream_resources.bootstrap_started_at
    END,
    retired_at = CASE
      WHEN replication_stream_resources.status = 3 THEN NULL
      ELSE replication_stream_resources.retired_at
    END,
    activated_at = CASE
      WHEN replication_stream_resources.status = 3 THEN NULL
      ELSE replication_stream_resources.activated_at
    END,
    error = CASE
      WHEN replication_stream_resources.status = 3 THEN NULL
      ELSE replication_stream_resources.error
    END,
    updated_at = excluded.updated_at;

-- resource_names is a JSON array of names to retain; [] retires every resource.
-- name: RetireReplicationStreamResources :exec
UPDATE replication_stream_resources
SET status = 3, retired_at = @retired_at, updated_at = @updated_at
WHERE replication_stream_id = @replication_stream_id
  AND status <> 3
  AND resource_name NOT IN (SELECT CAST(value AS TEXT) FROM json_each(CAST(@resource_names AS TEXT)));

-- name: ListReplicationStreamResources :many
SELECT *
FROM replication_stream_resources
WHERE replication_stream_id = @replication_stream_id
ORDER BY resource_name;
