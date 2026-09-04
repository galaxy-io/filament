-- name: CreatePipeline :exec
INSERT INTO pipelines (id, tenant_id, name, description, worker_configuration, created_at, updated_at)
VALUES (@pipeline_id, @tenant_id, @name, @description, coalesce(nullif(@worker_configuration, ''), '{}'), @created_at, @updated_at);

-- name: UpdatePipeline :execrows
UPDATE pipelines SET name = @name, description = @description,
  worker_configuration = coalesce(nullif(@worker_configuration, ''), worker_configuration),
  updated_at = @updated_at
WHERE tenant_id = @tenant_id AND id = @pipeline_id AND is_deleted = 0;

-- name: NextPipelineVersion :one
SELECT coalesce(max(v.version), 0) + 1 AS version
FROM pipelines p LEFT JOIN pipeline_versions v ON v.pipeline_id = p.id
WHERE p.tenant_id = @tenant_id AND p.id = @pipeline_id AND p.is_deleted = 0
GROUP BY p.id;

-- name: InsertPipelineVersion :one
INSERT INTO pipeline_versions (id, tenant_id, pipeline_id, version, graph, created_at, updated_at)
SELECT @version_id, p.tenant_id, @pipeline_id, @version, @graph, @created_at, @updated_at
FROM pipelines p WHERE p.tenant_id = @tenant_id AND p.id = @version_pipeline_id
RETURNING id, version, created_at;

-- name: SetCurrentPipelineVersion :exec
UPDATE pipelines SET current_version_id = @version_id, updated_at = @updated_at
WHERE tenant_id = @tenant_id AND id = @pipeline_id;

-- name: GetPipeline :one
SELECT id, tenant_id, name, description, current_version_id, worker_configuration,
       created_at, updated_at, deleted_at,
       coalesce(created_by_user_id, '') AS created_by_user_id,
       coalesce(updated_by_user_id, '') AS updated_by_user_id,
       coalesce(deleted_by_user_id, '') AS deleted_by_user_id
FROM pipelines WHERE tenant_id = @tenant_id AND id = @pipeline_id;

-- name: GetCurrentPipelineVersionID :one
SELECT coalesce(current_version_id, '') FROM pipelines WHERE tenant_id = @tenant_id AND id = @pipeline_id;

-- name: GetPipelineVersionByNumber :one
SELECT id, pipeline_id, version, graph, created_at, updated_at,
       coalesce(created_by_user_id, '') AS created_by_user_id,
       coalesce(updated_by_user_id, '') AS updated_by_user_id,
       coalesce(deleted_by_user_id, '') AS deleted_by_user_id
FROM pipeline_versions
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND version = @version;

-- name: GetPipelineVersionByID :one
SELECT id, pipeline_id, version, graph, created_at, updated_at,
       coalesce(created_by_user_id, '') AS created_by_user_id,
       coalesce(updated_by_user_id, '') AS updated_by_user_id,
       coalesce(deleted_by_user_id, '') AS deleted_by_user_id
FROM pipeline_versions
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND id = @version_id;

-- name: CountPipelineVersions :one
SELECT count(*) FROM pipeline_versions
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id;

-- name: CountPipelines :one
SELECT count(*) FROM pipelines
WHERE tenant_id = @tenant_id
  AND (cast(@include_deleted AS boolean) OR is_deleted = 0)
  AND (cast(@search AS text) = ''
       OR instr(lower(name), lower(@name_search)) > 0
       OR instr(lower(description), lower(@description_search)) > 0);

-- name: DeletePipeline :exec
UPDATE pipelines SET
  is_deleted = 1,
  deleted_at = @deleted_at,
  name = name || @delete_stamp,
  updated_at = @updated_at
WHERE tenant_id = @tenant_id AND id = @pipeline_id AND is_deleted = 0;
