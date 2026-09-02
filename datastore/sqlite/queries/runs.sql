-- Named parameters appear at most once per query: sqlc's sqlite engine leaves
-- repeated occurrences unbound, and SQLite silently binds them as NULL. The
-- generated-SQL tripwire in store_test.go enforces this.

-- name: SaveRun :execrows
INSERT INTO runs (id, tenant_id, pipeline_id, pipeline_version_id, schedule_id, status, request, records, bytes, scheduled_at, requested_at, started_at, ended_at, error, cpu_seconds, memory_peak_bytes, created_at, updated_at)
VALUES (@run_id, @tenant_id, nullif(@pipeline_id, ''), nullif(@pipeline_version_id, ''), nullif(@schedule_id, ''), @status, @request, @records, @bytes, @scheduled_at, @requested_at, @started_at, @ended_at, nullif(@error, ''), @cpu_seconds, @memory_peak_bytes, @created_at, @updated_at)
ON CONFLICT (id) DO UPDATE SET
    pipeline_id = excluded.pipeline_id,
    pipeline_version_id = excluded.pipeline_version_id,
    schedule_id = excluded.schedule_id,
    status = excluded.status,
    request = excluded.request,
    records = excluded.records,
    bytes = excluded.bytes,
    -- Lifecycle stamps are first-write-wins. Each is owned by exactly one
    -- module, so a save from any other must not roll it back. created_at is
    -- absent by design: it is birth.
    scheduled_at = coalesce(runs.scheduled_at, excluded.scheduled_at),
    requested_at = coalesce(runs.requested_at, excluded.requested_at),
    started_at = coalesce(runs.started_at, excluded.started_at),
    ended_at = coalesce(runs.ended_at, excluded.ended_at),
    error = excluded.error,
    cpu_seconds = excluded.cpu_seconds,
    memory_peak_bytes = excluded.memory_peak_bytes,
    updated_at = excluded.updated_at
WHERE runs.tenant_id = excluded.tenant_id;

-- PromoteScheduledRun is CreateRun's update arm: it only fires while the
-- existing row is still at @from_status, so a racing intake cannot roll a
-- live run back. The store tries it first and inserts when it touches no row.
-- name: PromoteScheduledRun :execrows
UPDATE runs SET
    pipeline_id = nullif(@pipeline_id, ''),
    pipeline_version_id = nullif(@pipeline_version_id, ''),
    schedule_id = nullif(@schedule_id, ''),
    status = @status,
    request = @request,
    records = @records,
    bytes = @bytes,
    scheduled_at = coalesce(runs.scheduled_at, @scheduled_at),
    requested_at = coalesce(runs.requested_at, @requested_at),
    started_at = coalesce(runs.started_at, @started_at),
    ended_at = coalesce(runs.ended_at, @ended_at),
    error = nullif(@error, ''),
    cpu_seconds = @cpu_seconds,
    memory_peak_bytes = @memory_peak_bytes,
    updated_at = @updated_at
WHERE tenant_id = @tenant_id AND id = @run_id AND status = @from_status;

-- name: RunExists :one
SELECT count(*) FROM runs WHERE tenant_id = @tenant_id AND id = @run_id;

-- name: InsertRun :execrows
INSERT INTO runs (id, tenant_id, pipeline_id, pipeline_version_id, schedule_id, status, request, records, bytes, scheduled_at, requested_at, started_at, ended_at, error, cpu_seconds, memory_peak_bytes, created_at, updated_at)
VALUES (@run_id, @tenant_id, nullif(@pipeline_id, ''), nullif(@pipeline_version_id, ''), nullif(@schedule_id, ''), @status, @request, @records, @bytes, @scheduled_at, @requested_at, @started_at, @ended_at, nullif(@error, ''), @cpu_seconds, @memory_peak_bytes, @created_at, @updated_at)
ON CONFLICT (id) DO NOTHING;

-- name: DeleteRun :exec
DELETE FROM runs WHERE tenant_id = @tenant_id AND id = @run_id;

-- name: GetRunStatus :one
SELECT status FROM runs WHERE tenant_id = @tenant_id AND id = @run_id;

-- name: TransitionRun :exec
UPDATE runs SET
    status = @status,
    ended_at = CASE WHEN cast(@stamp_ended AS boolean) THEN coalesce(ended_at, @ended_now) ELSE ended_at END,
    updated_at = @updated_at
WHERE tenant_id = @tenant_id AND id = @run_id;

-- name: ResetRunExecution :exec
UPDATE runs SET
    status = @status,
    records = CASE WHEN cast(@preserve_progress AS boolean) THEN records ELSE 0 END,
    bytes = CASE WHEN cast(@preserve_progress AS boolean) THEN bytes ELSE 0 END,
    cpu_seconds = 0,
    memory_peak_bytes = 0,
    requested_at = @requested_at,
    started_at = NULL,
    ended_at = NULL,
    error = NULL,
    updated_at = @updated_at
WHERE tenant_id = @tenant_id AND id = @run_id;

-- Reaps a pipeline's pre-created scheduled runs by the pipeline_id column:
-- deleting the schedules row SET-NULLs runs.schedule_id, so schedule-scoped
-- lookups cannot find these rows once the delete tx is underway.
-- name: DeletePipelineScheduledRuns :exec
DELETE FROM runs WHERE tenant_id = @tenant_id AND pipeline_id = @pipeline_id AND status = @status;

-- Reaps a schedule's pre-created scheduled runs; must run before the schedules
-- row is deleted, since that delete SET-NULLs runs.schedule_id.
-- name: DeleteScheduleScheduledRuns :exec
DELETE FROM runs WHERE tenant_id = @tenant_id AND schedule_id = @schedule_id AND status = @status;

-- name: LoadRun :one
SELECT id, tenant_id, coalesce(schedule_id, '') AS schedule_id, status, request, records, bytes, created_at, scheduled_at, requested_at, started_at, ended_at, updated_at, coalesce(error, '') AS error, cpu_seconds, memory_peak_bytes
FROM runs WHERE tenant_id = @tenant_id AND id = @run_id;
