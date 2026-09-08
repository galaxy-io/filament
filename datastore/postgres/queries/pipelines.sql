-- name: CreatePipeline :one
INSERT INTO pipelines (id, tenant_id, name, description, worker_configuration, updated_at)
VALUES (@pipeline_id, @tenant_id, @name, @description, COALESCE(sqlc.narg(worker_configuration)::jsonb, '{}'::jsonb), now())
RETURNING created_at;

-- name: UpdatePipeline :execrows
UPDATE pipelines SET name = @name, description = @description,
  worker_configuration = COALESCE(sqlc.narg(worker_configuration)::jsonb, worker_configuration),
  updated_at = now()
WHERE tenant_id = @tenant_id AND id = @pipeline_id AND NOT is_deleted;

-- name: CreatePipelineVersion :one
WITH locked AS MATERIALIZED (
  SELECT id FROM pipelines
  WHERE tenant_id = sqlc.arg(tenant_id) AND id = sqlc.arg(pipeline_id) AND NOT is_deleted
  FOR UPDATE
), next AS (
	SELECT locked.id AS pipeline_id, coalesce(max(v.version), 0) + 1 AS version
	FROM locked LEFT JOIN pipeline_versions v ON v.pipeline_id = locked.id
	GROUP BY locked.id
), inserted AS (
	INSERT INTO pipeline_versions (id, tenant_id, pipeline_id, version, graph)
	SELECT sqlc.arg(id), p.tenant_id, sqlc.arg(pipeline_id), next.version, sqlc.arg(graph)
	FROM next
	JOIN pipelines p ON p.tenant_id = sqlc.arg(tenant_id) AND p.id = next.pipeline_id
  RETURNING id, version, created_at
)
UPDATE pipelines p SET current_version_id = inserted.id, updated_at = now()
FROM inserted WHERE p.tenant_id = sqlc.arg(tenant_id) AND p.id = sqlc.arg(pipeline_id)
RETURNING inserted.id, inserted.version, inserted.created_at;

-- name: GetPipeline :one
SELECT id, tenant_id, name, description, current_version_id, worker_configuration,
       created_at, updated_at, deleted_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id
FROM pipelines WHERE tenant_id = @tenant_id AND id = @pipeline_id;

-- name: GetPipelineVersion :one
SELECT id, pipeline_id, version, graph, created_at, updated_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id FROM pipeline_versions
WHERE pipeline_versions.pipeline_id = sqlc.arg(pipeline_id)
  AND pipeline_versions.tenant_id = sqlc.arg(tenant_id)
  AND ((sqlc.arg(version)::bigint = 0 AND pipeline_versions.id = (SELECT current_version_id FROM pipelines WHERE pipelines.tenant_id = sqlc.arg(tenant_id) AND pipelines.id = sqlc.arg(pipeline_id)))
    OR pipeline_versions.version = sqlc.arg(version));

-- name: ListPipelineVersions :many
SELECT id, pipeline_id, version, graph, created_at, updated_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id FROM pipeline_versions
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id
ORDER BY
  CASE WHEN @sort_by::text IN ('', 'version') AND NOT @sort_desc::boolean THEN version END ASC,
  CASE WHEN @sort_by::text IN ('', 'version') AND @sort_desc::boolean THEN version END DESC,
  CASE WHEN @sort_by::text = 'created_at' AND NOT @sort_desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_by::text = 'created_at' AND @sort_desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_by::text = 'updated_at' AND NOT @sort_desc::boolean THEN updated_at END ASC,
  CASE WHEN @sort_by::text = 'updated_at' AND @sort_desc::boolean THEN updated_at END DESC,
  CASE WHEN NOT @sort_desc::boolean THEN id END ASC,
  CASE WHEN @sort_desc::boolean THEN id END DESC
LIMIT NULLIF(@lim::int, 0)
OFFSET @offset_rows::int;

-- name: CountPipelineVersions :one
SELECT count(*) FROM pipeline_versions
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id;

-- name: ListPipelines :many
SELECT id, tenant_id, name, description, current_version_id, worker_configuration,
       created_at, updated_at, deleted_at,
       created_by_user_id, updated_by_user_id, deleted_by_user_id
FROM pipelines
WHERE tenant_id = @tenant_id
  AND (@include_deleted::boolean OR NOT is_deleted)
  AND (nullif(@search::text, '') IS NULL
       OR name ILIKE '%' || @search || '%' ESCAPE '\'
       OR description ILIKE '%' || @search || '%' ESCAPE '\')
ORDER BY
  CASE WHEN @sort_by::text IN ('', 'id') AND NOT @sort_desc::boolean THEN id END ASC,
  CASE WHEN @sort_by::text IN ('', 'id') AND @sort_desc::boolean THEN id END DESC,
  CASE WHEN @sort_by::text = 'name' AND NOT @sort_desc::boolean THEN lower(name) END ASC,
  CASE WHEN @sort_by::text = 'name' AND @sort_desc::boolean THEN lower(name) END DESC,
  CASE WHEN @sort_by::text = 'created_at' AND NOT @sort_desc::boolean THEN created_at END ASC,
  CASE WHEN @sort_by::text = 'created_at' AND @sort_desc::boolean THEN created_at END DESC,
  CASE WHEN @sort_by::text = 'updated_at' AND NOT @sort_desc::boolean THEN updated_at END ASC,
  CASE WHEN @sort_by::text = 'updated_at' AND @sort_desc::boolean THEN updated_at END DESC,
  CASE WHEN NOT @sort_desc::boolean THEN id END ASC,
  CASE WHEN @sort_desc::boolean THEN id END DESC
LIMIT NULLIF(@lim::int, 0)
OFFSET @offset_rows::int;

-- name: CountPipelines :one
SELECT count(*) FROM pipelines
WHERE tenant_id = @tenant_id
  AND (@include_deleted::boolean OR NOT is_deleted)
  AND (nullif(@search::text, '') IS NULL
       OR name ILIKE '%' || @search || '%' ESCAPE '\'
       OR description ILIKE '%' || @search || '%' ESCAPE '\');

-- name: DeletePipeline :exec
UPDATE pipelines SET
  is_deleted = true,
  deleted_at = now(),
  name = name || '__deleted__' || to_char(now() AT TIME ZONE 'UTC', 'YYYY-MM-DD"T"HH24:MI:SS.MS"Z"'),
  updated_at = now()
WHERE tenant_id = @tenant_id AND id = @pipeline_id AND NOT is_deleted;
