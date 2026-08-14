-- name: CreatePipeline :one
INSERT INTO pipelines (id, tenant_id, name, description, updated_at)
VALUES (@pipeline_id, @tenant_id, @name, @description, now())
RETURNING created_at;

-- name: UpdatePipeline :execrows
UPDATE pipelines SET name = @name, description = @description, updated_at = now()
WHERE id = @pipeline_id AND NOT is_deleted;

-- name: CreatePipelineVersion :one
WITH locked AS MATERIALIZED (
  SELECT id FROM pipelines WHERE id = sqlc.arg(pipeline_id) AND NOT is_deleted FOR UPDATE
), next AS (
  SELECT coalesce(max(v.version), 0) + 1 AS version
  FROM locked LEFT JOIN pipeline_versions v ON v.pipeline_id = locked.id
), inserted AS (
  INSERT INTO pipeline_versions (id, pipeline_id, version, graph)
  SELECT sqlc.arg(id), sqlc.arg(pipeline_id), version, sqlc.arg(graph) FROM next
  RETURNING id, version, created_at
)
UPDATE pipelines p SET current_version_id = inserted.id, updated_at = now()
FROM inserted WHERE p.id = sqlc.arg(pipeline_id)
RETURNING inserted.id, inserted.version, inserted.created_at;

-- name: GetPipeline :one
SELECT id, tenant_id, name, description, current_version_id, created_at, updated_at, deleted_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id
FROM pipelines WHERE id = @pipeline_id;

-- name: GetPipelineVersion :one
SELECT id, pipeline_id, version, graph, created_at, updated_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id FROM pipeline_versions
WHERE pipeline_id = sqlc.arg(pipeline_id)
  AND ((sqlc.arg(version)::bigint = 0 AND id = (SELECT current_version_id FROM pipelines WHERE pipelines.id = sqlc.arg(pipeline_id)))
    OR version = sqlc.arg(version));

-- name: ListPipelineVersions :many
SELECT id, pipeline_id, version, graph, created_at, updated_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id FROM pipeline_versions
WHERE pipeline_id = @pipeline_id ORDER BY version DESC;

-- name: ListPipelines :many
SELECT id, tenant_id, name, description, current_version_id, created_at, updated_at, deleted_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id
FROM pipelines
WHERE (@tenant_id::text = '' OR tenant_id = @tenant_id)
  AND (@include_deleted::boolean OR NOT is_deleted)
ORDER BY id;

-- name: DeletePipeline :exec
UPDATE pipelines SET
  is_deleted = true,
  deleted_at = now(),
  name = name || '__deleted__' || to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
  updated_at = now()
WHERE id = @pipeline_id AND NOT is_deleted;
