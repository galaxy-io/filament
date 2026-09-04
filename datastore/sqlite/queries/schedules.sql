-- The insert arm sources from pipelines so a save racing a pipeline delete
-- cannot re-insert the schedule row the delete just removed; 0 rows means the
-- pipeline is gone or deleted.
-- name: SaveSchedule :execrows
INSERT INTO schedules (id, tenant_id, pipeline_id, name, cron_expr, timezone, overlap_policy,
    enabled, last_fired_at, next_fire_at, claimed_at, created_at, updated_at)
SELECT @schedule_id, @tenant_id, @pipeline_id, @name, @cron_expr, @timezone, @overlap_policy,
    @enabled, @last_fired_at, @next_fire_at, NULL, @created_at, @updated_at
FROM pipelines WHERE pipelines.tenant_id = @pipeline_tenant_id AND pipelines.id = @schedule_pipeline_id AND pipelines.is_deleted = 0
ON CONFLICT (id) DO UPDATE SET
    pipeline_id = excluded.pipeline_id,
    name = excluded.name,
    cron_expr = excluded.cron_expr,
    timezone = excluded.timezone,
    overlap_policy = excluded.overlap_policy,
    enabled = excluded.enabled,
    last_fired_at = excluded.last_fired_at,
    next_fire_at = excluded.next_fire_at,
    claimed_at = NULL,
    updated_at = excluded.updated_at
WHERE schedules.tenant_id = excluded.tenant_id;

-- name: LoadSchedule :one
SELECT id, tenant_id, pipeline_id, name, cron_expr, timezone, overlap_policy,
    enabled, last_fired_at, next_fire_at, created_at
FROM schedules WHERE tenant_id = @tenant_id AND id = @schedule_id;

-- name: LoadPipelineSchedule :one
SELECT id, tenant_id, pipeline_id, name, cron_expr, timezone, overlap_policy,
    enabled, last_fired_at, next_fire_at, created_at
FROM schedules WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id;

-- name: DeleteSchedule :exec
DELETE FROM schedules WHERE tenant_id = @tenant_id AND id = @schedule_id;

-- name: DeletePipelineSchedules :exec
DELETE FROM schedules WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id;

-- ListSchedules filters enabled tri-state in Go: filter_enabled is -1 for
-- no filter, else 0/1. The limit is resolved in Go (-1 means unlimited).
-- name: ListSchedules :many
SELECT id, tenant_id, pipeline_id, name, cron_expr, timezone, overlap_policy,
    enabled, last_fired_at, next_fire_at, created_at
FROM schedules
WHERE tenant_id = @tenant_id
  AND (cast(@filter_enabled AS integer) < 0 OR enabled = @enabled)
ORDER BY id
LIMIT @lim;

-- ClaimDue runs inside a transaction with LeaseSchedule; the single-writer
-- connection provides the serialization Postgres gets from FOR UPDATE SKIP
-- LOCKED. lease_cutoff is now - leaseTTL, computed in Go: a claimed_at older
-- than that is an abandoned lease, eligible to be reclaimed.
-- name: ClaimDue :many
SELECT id, tenant_id, pipeline_id, name, cron_expr, timezone, overlap_policy,
    enabled, last_fired_at, next_fire_at, created_at
FROM schedules
WHERE enabled = 1 AND next_fire_at <= @now
  AND (claimed_at IS NULL OR claimed_at < @lease_cutoff)
ORDER BY next_fire_at
LIMIT @lim;

-- name: LeaseSchedule :exec
UPDATE schedules SET claimed_at = @claimed_at WHERE id = @schedule_id;

-- name: ReleaseScheduleClaim :exec
UPDATE schedules SET claimed_at = NULL, updated_at = @updated_at
WHERE tenant_id = @tenant_id AND id = @schedule_id;
