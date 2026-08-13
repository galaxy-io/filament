-- name: CreatePipeline :one
INSERT INTO pipelines (pipeline_id, tenant_id, name, description, current_version_id, worker_configuration, updated_at)
VALUES (@pipeline_id, @tenant_id, @name, @description, 0, COALESCE(sqlc.narg(worker_configuration)::jsonb, '{}'::jsonb), now())
RETURNING created_at;

-- name: UpdatePipeline :execrows
UPDATE pipelines SET name = @name, description = @description,
  worker_configuration = COALESCE(sqlc.narg(worker_configuration)::jsonb, worker_configuration),
  updated_at = now()
WHERE pipeline_id = @pipeline_id AND NOT is_deleted;

-- name: CreatePipelineVersion :one
WITH next AS (
  SELECT current_version_id + 1 AS version FROM pipelines p WHERE p.pipeline_id = sqlc.arg(pipeline_id) AND NOT p.is_deleted FOR UPDATE
), inserted AS (
  INSERT INTO pipeline_versions (pipeline_id, version, nodes, edges)
  SELECT sqlc.arg(pipeline_id), version, sqlc.arg(nodes), sqlc.arg(edges) FROM next
  RETURNING version, created_at
)
UPDATE pipelines p SET current_version_id = inserted.version, updated_at = now()
FROM inserted WHERE p.pipeline_id = sqlc.arg(pipeline_id)
RETURNING inserted.version, inserted.created_at;

-- name: GetPipeline :one
SELECT pipeline_id, tenant_id, name, description, current_version_id, last_run_version_id,
       last_run_at, last_run_status, last_run_bytes, last_run_ended_at, created_at, deleted_at,
       worker_configuration
FROM pipelines WHERE pipeline_id = @pipeline_id;

-- name: GetPipelineVersion :one
SELECT pipeline_id, version, nodes, edges, created_at FROM pipeline_versions
WHERE pipeline_versions.pipeline_id = sqlc.arg(pipeline_id) AND version = CASE WHEN sqlc.arg(version)::bigint = 0 THEN
  (SELECT current_version_id FROM pipelines WHERE pipelines.pipeline_id = sqlc.arg(pipeline_id)) ELSE sqlc.arg(version) END;

-- name: ListPipelineVersions :many
SELECT pipeline_id, version, nodes, edges, created_at FROM pipeline_versions
WHERE pipeline_id = @pipeline_id ORDER BY version DESC;

-- name: ListPipelines :many
SELECT pipeline_id, tenant_id, name, description, current_version_id, last_run_version_id,
       last_run_at, last_run_status, last_run_bytes, last_run_ended_at, created_at, deleted_at,
       worker_configuration
FROM pipelines
WHERE (@tenant_id::text = '' OR tenant_id = @tenant_id)
  AND (@include_deleted::boolean OR NOT is_deleted)
ORDER BY pipeline_id;

-- name: UpdatePipelineRunSummary :exec
UPDATE pipelines SET
  last_run_version_id = @version,
  last_run_at = @started_at,
  last_run_status = @status,
  last_run_bytes = @bytes,
  last_run_ended_at = @ended_at,
  updated_at = now()
WHERE pipeline_id = @pipeline_id AND (last_run_at IS NULL OR last_run_at <= @started_at);

-- name: DeletePipeline :exec
UPDATE pipelines SET
  is_deleted = true,
  deleted_at = now(),
  name = name || '__deleted__' || to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
  updated_at = now()
WHERE pipeline_id = @pipeline_id AND NOT is_deleted;
