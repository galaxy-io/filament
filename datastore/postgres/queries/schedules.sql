-- name: SaveSchedule :exec
INSERT INTO schedules (schedule_id, tenant_id, name, cron_expr, timezone, jitter_ms, overlap_policy, catchup_policy,
    request, enabled, last_fired_at, next_fire_at, claimed_at, last_run_id, last_run_status, created_at, updated_at)
VALUES (@schedule_id, @tenant_id, @name, @cron_expr, @timezone, @jitter_ms, @overlap_policy, @catchup_policy,
    @request, @enabled, @last_fired_at, @next_fire_at, NULL, nullif(@last_run_id::text, ''), @last_run_status, @created_at, now())
ON CONFLICT (schedule_id) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
    name = EXCLUDED.name,
    cron_expr = EXCLUDED.cron_expr,
    timezone = EXCLUDED.timezone,
    jitter_ms = EXCLUDED.jitter_ms,
    overlap_policy = EXCLUDED.overlap_policy,
    catchup_policy = EXCLUDED.catchup_policy,
    request = EXCLUDED.request,
    enabled = EXCLUDED.enabled,
    last_fired_at = EXCLUDED.last_fired_at,
    next_fire_at = EXCLUDED.next_fire_at,
    claimed_at = NULL,
    last_run_id = EXCLUDED.last_run_id,
    last_run_status = EXCLUDED.last_run_status,
    updated_at = now();

-- name: LoadSchedule :one
SELECT schedule_id, tenant_id, name, cron_expr, timezone, jitter_ms, overlap_policy, catchup_policy,
    request, enabled, last_fired_at, next_fire_at, coalesce(last_run_id, '')::text AS last_run_id, last_run_status, created_at
FROM schedules WHERE schedule_id = @schedule_id;

-- name: DeleteSchedule :exec
DELETE FROM schedules WHERE schedule_id = @schedule_id;

-- name: ListSchedules :many
SELECT schedule_id, tenant_id, name, cron_expr, timezone, jitter_ms, overlap_policy, catchup_policy,
    request, enabled, last_fired_at, next_fire_at, coalesce(last_run_id, '')::text AS last_run_id, last_run_status, created_at
FROM schedules
WHERE (@tenant_id::text = '' OR tenant_id = @tenant_id)
  AND (sqlc.narg(filter_enabled)::boolean IS NULL OR enabled = sqlc.narg(filter_enabled))
ORDER BY schedule_id
LIMIT NULLIF(@lim::int, 0);

-- name: ClaimDue :many
-- lease_cutoff is now - leaseTTL, computed in Go: a claimed_at older than that
-- is treated as an abandoned lease and eligible to be reclaimed.
SELECT schedule_id, tenant_id, name, cron_expr, timezone, jitter_ms, overlap_policy, catchup_policy,
    request, enabled, last_fired_at, next_fire_at, coalesce(last_run_id, '')::text AS last_run_id, last_run_status, created_at
FROM schedules
WHERE enabled AND next_fire_at <= @now
  AND (claimed_at IS NULL OR claimed_at < @lease_cutoff::timestamptz)
ORDER BY next_fire_at
LIMIT NULLIF(@lim::int, 0)
FOR UPDATE SKIP LOCKED;

-- name: LeaseSchedules :exec
UPDATE schedules SET claimed_at = @claimed_at WHERE schedule_id = ANY(@schedule_ids::text[]);

-- name: MarkFired :exec
UPDATE schedules SET last_fired_at = @fired_at, claimed_at = NULL, updated_at = now() WHERE schedule_id = @schedule_id;
