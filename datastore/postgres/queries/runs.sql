-- name: SaveRun :exec
INSERT INTO runs (id, tenant_id, pipeline_id, pipeline_version_id, schedule_id, status, request, records, bytes, scheduled_at, requested_at, started_at, ended_at, error, cpu_seconds, memory_peak_bytes, updated_at)
VALUES (@run_id, @tenant_id, nullif(@pipeline_id::text, ''), nullif(@pipeline_version_id::text, ''), nullif(@schedule_id::text, ''), @status, @request, @records, @bytes, @scheduled_at, @requested_at, @started_at, @ended_at, nullif(@error::text, ''), @cpu_seconds, @memory_peak_bytes, now())
ON CONFLICT (id) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
    pipeline_id = EXCLUDED.pipeline_id,
    pipeline_version_id = EXCLUDED.pipeline_version_id,
    schedule_id = EXCLUDED.schedule_id,
    status = EXCLUDED.status,
    request = EXCLUDED.request,
    records = EXCLUDED.records,
    bytes = EXCLUDED.bytes,
    -- Lifecycle stamps are first-write-wins. Each is owned by exactly one
    -- module, so a save from any other must not roll it back — that makes the
    -- ordering guarantee a property of the store rather than of every caller
    -- remembering to check. created_at is absent by design: it is birth.
    scheduled_at = coalesce(runs.scheduled_at, EXCLUDED.scheduled_at),
    requested_at = coalesce(runs.requested_at, EXCLUDED.requested_at),
    started_at = coalesce(runs.started_at, EXCLUDED.started_at),
    ended_at = coalesce(runs.ended_at, EXCLUDED.ended_at),
    error = EXCLUDED.error,
    cpu_seconds = EXCLUDED.cpu_seconds,
    memory_peak_bytes = EXCLUDED.memory_peak_bytes,
    updated_at = now();

-- CreateRun inserts a run or promotes a pre-created scheduled row. The update
-- arm only fires while the existing row is still at @from_status, so a racing
-- intake cannot roll a live run back; 0 rows reports the conflict.
-- name: CreateRun :execrows
INSERT INTO runs (id, tenant_id, pipeline_id, pipeline_version_id, schedule_id, status, request, records, bytes, scheduled_at, requested_at, started_at, ended_at, error, cpu_seconds, memory_peak_bytes, updated_at)
VALUES (@run_id, @tenant_id, nullif(@pipeline_id::text, ''), nullif(@pipeline_version_id::text, ''), nullif(@schedule_id::text, ''), @status, @request, @records, @bytes, @scheduled_at, @requested_at, @started_at, @ended_at, nullif(@error::text, ''), @cpu_seconds, @memory_peak_bytes, now())
ON CONFLICT (id) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
    pipeline_id = EXCLUDED.pipeline_id,
    pipeline_version_id = EXCLUDED.pipeline_version_id,
    schedule_id = EXCLUDED.schedule_id,
    status = EXCLUDED.status,
    request = EXCLUDED.request,
    records = EXCLUDED.records,
    bytes = EXCLUDED.bytes,
    scheduled_at = coalesce(runs.scheduled_at, EXCLUDED.scheduled_at),
    requested_at = coalesce(runs.requested_at, EXCLUDED.requested_at),
    started_at = coalesce(runs.started_at, EXCLUDED.started_at),
    ended_at = coalesce(runs.ended_at, EXCLUDED.ended_at),
    error = EXCLUDED.error,
    cpu_seconds = EXCLUDED.cpu_seconds,
    memory_peak_bytes = EXCLUDED.memory_peak_bytes,
    updated_at = now()
WHERE runs.status = @from_status;

-- name: DeleteRun :exec
DELETE FROM runs WHERE id = @run_id;

-- name: LoadRun :one
SELECT id, tenant_id, coalesce(schedule_id, '')::text AS schedule_id, status, request, records, bytes, created_at, scheduled_at, requested_at, started_at, ended_at, updated_at, coalesce(error, '')::text AS error, cpu_seconds, memory_peak_bytes
FROM runs WHERE id = @run_id;
