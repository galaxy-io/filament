-- The first statement in a notifier write transaction takes the SQLite write
-- lock before any reads, including when another process opens the same file.
-- name: LockNotifierPipeline :one
UPDATE pipelines SET id = id WHERE tenant_id = @tenant_id AND id = @pipeline_id AND is_deleted = 0 RETURNING id;

-- name: CreateNotifier :one
INSERT INTO notifier (id, tenant_id, pipeline_id, name, notification_type, is_enabled,
  events, resources, config, secret_refs, created_by_user_id, updated_by_user_id, created_at, updated_at)
VALUES (@notifier_id, @tenant_id, @pipeline_id, @name, @notification_type, @is_enabled,
  @events, @resources, @config, @secret_refs, sqlc.narg(created_by_user_id), sqlc.narg(updated_by_user_id), @created_at, @updated_at)
RETURNING *;

-- name: GetNotifier :one
SELECT * FROM notifier
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND id = @notifier_id;

-- name: ListNotifiers :many
SELECT * FROM notifier
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id
  AND (cast(@include_deleted AS boolean) OR is_deleted = 0)
ORDER BY id;

-- name: UpdateNotifier :one
UPDATE notifier SET name = @name, is_enabled = @is_enabled,
  events = @events, resources = @resources, config = @config, secret_refs = @secret_refs,
  version = version + 1, updated_at = @updated_at, updated_by_user_id = sqlc.narg(updated_by_user_id)
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND id = @notifier_id
  AND version = @expected_version AND is_deleted = 0
RETURNING *;

-- name: DeleteNotifier :one
UPDATE notifier SET is_deleted = 1, deleted_at = @deleted_at, updated_at = @updated_at, version = version + 1
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND id = @notifier_id
  AND version = @expected_version AND is_deleted = 0
RETURNING *;

-- name: DeletePipelineNotifiers :exec
UPDATE notifier SET is_deleted = 1, deleted_at = @deleted_at, updated_at = @updated_at, version = version + 1
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND is_deleted = 0;
