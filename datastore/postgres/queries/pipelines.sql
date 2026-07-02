-- name: CreatePipeline :exec
INSERT INTO pipelines (pipeline_id, tenant_id, name, nodes, edges, version, updated_at)
VALUES (@pipeline_id, @tenant_id, @name, @nodes, @edges, 1, now());

-- name: UpdatePipeline :one
UPDATE pipelines SET name = @name, nodes = @nodes, edges = @edges, version = version + 1, updated_at = now()
WHERE pipeline_id = @pipeline_id AND version = @expected_version
RETURNING version;

-- name: GetPipeline :one
SELECT pipeline_id, tenant_id, name, nodes, edges, version FROM pipelines WHERE pipeline_id = @pipeline_id;

-- name: ListPipelines :many
SELECT pipeline_id, tenant_id, name, nodes, edges, version FROM pipelines
WHERE (@tenant_id::text = '' OR tenant_id = @tenant_id)
ORDER BY pipeline_id;

-- name: DeletePipeline :exec
DELETE FROM pipelines WHERE pipeline_id = @pipeline_id;
