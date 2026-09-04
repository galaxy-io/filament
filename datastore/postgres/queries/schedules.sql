-- The insert arm sources from pipelines so a save racing a pipeline delete
-- cannot re-insert the schedule row the delete just removed; 0 rows means the
-- pipeline is gone or deleted.
-- name: SaveSchedule :execrows
INSERT INTO schedules (id, tenant_id, pipeline_id, name, cron_expr, timezone, overlap_policy,
    enabled, last_fired_at, next_fire_at, claimed_at, created_at, updated_at)
SELECT @schedule_id, @tenant_id, @pipeline_id, @name, @cron_expr, @timezone, @overlap_policy,
    @enabled, @last_fired_at, @next_fire_at, NULL, @created_at, now()
FROM pipelines WHERE tenant_id = @tenant_id AND id = @pipeline_id AND NOT is_deleted
ON CONFLICT (id) DO UPDATE SET
    pipeline_id = EXCLUDED.pipeline_id,
    name = EXCLUDED.name,
    cron_expr = EXCLUDED.cron_expr,
    timezone = EXCLUDED.timezone,
    overlap_policy = EXCLUDED.overlap_policy,
    enabled = EXCLUDED.enabled,
    last_fired_at = EXCLUDED.last_fired_at,
    next_fire_at = EXCLUDED.next_fire_at,
    claimed_at = NULL,
    updated_at = now()
WHERE schedules.tenant_id = EXCLUDED.tenant_id;

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

-- name: ListSchedules :many
SELECT id, tenant_id, pipeline_id, name, cron_expr, timezone, overlap_policy,
    enabled, last_fired_at, next_fire_at, created_at
FROM schedules
WHERE tenant_id = @tenant_id
  AND (sqlc.narg(filter_enabled)::boolean IS NULL OR enabled = sqlc.narg(filter_enabled))
ORDER BY id
LIMIT NULLIF(@lim::int, 0);

-- name: ClaimDue :many
-- lease_cutoff is now - leaseTTL, computed in Go: a claimed_at older than that
-- is treated as an abandoned lease and eligible to be reclaimed.
SELECT id, tenant_id, pipeline_id, name, cron_expr, timezone, overlap_policy,
    enabled, last_fired_at, next_fire_at, created_at
FROM schedules
WHERE enabled AND next_fire_at <= @now
  AND (claimed_at IS NULL OR claimed_at < @lease_cutoff::timestamptz)
ORDER BY next_fire_at
LIMIT NULLIF(@lim::int, 0)
FOR UPDATE SKIP LOCKED;

-- name: LeaseSchedules :exec
UPDATE schedules SET claimed_at = @claimed_at WHERE id = ANY(@schedule_ids::uuid[]);

-- name: ReleaseScheduleClaim :exec
UPDATE schedules SET claimed_at = NULL, updated_at = now()
WHERE tenant_id = @tenant_id AND id = @schedule_id;
