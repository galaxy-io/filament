-- name: LockPipelineForReplicationStream :one
SELECT id
FROM pipelines
WHERE id = @pipeline_id
  AND tenant_id = @tenant_id
  AND deleted_at IS NULL
FOR UPDATE;

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

-- name: NextReplicationStreamGeneration :one
SELECT (COALESCE(MAX(generation), 0) + 1)::bigint
FROM replication_streams
WHERE pipeline_id = @pipeline_id
  AND route_key = @route_key;

-- name: RetireReplicationStream :exec
UPDATE replication_streams
SET status = 1, retired_at = now(), error = NULL, updated_at = now()
WHERE id = @replication_stream_id
  AND status = 0;

-- name: CreateReplicationStream :one
INSERT INTO replication_streams (
  id, tenant_id, pipeline_id, route_key, generation,
  source_connection_id, sink_connection_id,
  consumer_name, consumer_config, continuity_fingerprint,
  status, created_from_pipeline_version_id
) VALUES (
  @replication_stream_id, @tenant_id, @pipeline_id, @route_key, @generation,
  @source_connection_id, @sink_connection_id,
  @consumer_name, @consumer_config, @continuity_fingerprint,
  0, @created_from_pipeline_version_id
)
RETURNING *;

-- name: LockReplicationStream :one
SELECT id
FROM replication_streams
WHERE id = @replication_stream_id
  AND tenant_id = @tenant_id
  AND status = 0
FOR UPDATE;

-- name: UpsertReplicationStreamResource :exec
INSERT INTO replication_stream_resources (
  replication_stream_id, tenant_id, resource_name, status,
  bootstrap_mode, bootstrap_config
) VALUES (
  @replication_stream_id, @tenant_id, @resource_name, 0,
  @bootstrap_mode, @bootstrap_config
)
ON CONFLICT (replication_stream_id, resource_name) DO UPDATE
SET status = CASE
      WHEN replication_stream_resources.status = 3 THEN 0
      ELSE replication_stream_resources.status
    END,
    bootstrap_mode = EXCLUDED.bootstrap_mode,
    bootstrap_config = EXCLUDED.bootstrap_config,
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
    updated_at = now();

-- name: RetireReplicationStreamResources :exec
UPDATE replication_stream_resources
SET status = 3, retired_at = now(), updated_at = now()
WHERE replication_stream_id = @replication_stream_id
  AND status <> 3
  AND NOT (resource_name = ANY(@resource_names::text[]));

-- name: ListReplicationStreamResources :many
SELECT *
FROM replication_stream_resources
WHERE replication_stream_id = @replication_stream_id
ORDER BY resource_name;
