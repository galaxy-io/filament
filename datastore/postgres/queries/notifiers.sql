-- name: LockNotifierPipeline :one
SELECT id FROM pipelines WHERE tenant_id = @tenant_id AND id = @pipeline_id AND NOT is_deleted FOR UPDATE;

-- name: CreateNotifier :one
INSERT INTO notifier (id, tenant_id, pipeline_id, name, notification_type, enabled,
  events, resources, config, secret_refs, created_by_user_id, updated_by_user_id, created_at, updated_at)
VALUES (@notifier_id, @tenant_id, @pipeline_id, @name, @notification_type, @enabled,
  @events, @resources, @config, @secret_refs, sqlc.narg(created_by_user_id), sqlc.narg(updated_by_user_id), now(), now())
RETURNING *;

-- name: GetNotifier :one
SELECT * FROM notifier
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND id = @notifier_id;

-- name: ListNotifiers :many
SELECT * FROM notifier
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id
  AND (@include_deleted::boolean OR NOT is_deleted)
ORDER BY id;

-- name: UpdateNotifier :one
UPDATE notifier SET name = @name, enabled = @enabled,
  events = @events, resources = @resources, config = @config, secret_refs = @secret_refs,
  version = version + 1, updated_at = now(), updated_by_user_id = sqlc.narg(updated_by_user_id)
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND id = @notifier_id
  AND version = @expected_version AND NOT is_deleted
RETURNING *;

-- name: DeleteNotifier :one
UPDATE notifier SET is_deleted = true, deleted_at = now(), updated_at = now(), version = version + 1
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND id = @notifier_id
  AND version = @expected_version AND NOT is_deleted
RETURNING *;

-- name: DeletePipelineNotifiers :exec
UPDATE notifier SET is_deleted = true, deleted_at = now(), updated_at = now(), version = version + 1
WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND NOT is_deleted;
