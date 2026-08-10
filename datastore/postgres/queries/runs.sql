-- name: SaveRun :exec
INSERT INTO runs (run_id, tenant_id, schedule_id, status, request, records, bytes, scheduled_at, requested_at, started_at, finished_at, error, cpu_seconds, memory_peak_bytes, updated_at)
VALUES (@run_id, @tenant_id, nullif(@schedule_id::text, ''), @status, @request, @records, @bytes, @scheduled_at, @requested_at, @started_at, @finished_at, nullif(@error::text, ''), @cpu_seconds, @memory_peak_bytes, now())
ON CONFLICT (run_id) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
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
    finished_at = coalesce(runs.finished_at, EXCLUDED.finished_at),
    error = EXCLUDED.error,
    cpu_seconds = EXCLUDED.cpu_seconds,
    memory_peak_bytes = EXCLUDED.memory_peak_bytes,
    updated_at = now();

-- name: DeleteRun :exec
DELETE FROM runs WHERE run_id = @run_id;

-- name: LoadRun :one
SELECT run_id, tenant_id, coalesce(schedule_id, '')::text AS schedule_id, status, request, records, bytes, created_at, scheduled_at, requested_at, started_at, finished_at, updated_at, coalesce(error, '')::text AS error, cpu_seconds, memory_peak_bytes
FROM runs WHERE run_id = @run_id;
