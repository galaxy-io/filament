-- name: SaveRun :exec
INSERT INTO runs (run_id, tenant_id, schedule_id, status, request, records, bytes, started_at, finished_at, error, updated_at)
VALUES (@run_id, @tenant_id, nullif(@schedule_id::text, ''), @status, @request, @records, @bytes, @started_at, @finished_at, nullif(@error::text, ''), now())
ON CONFLICT (run_id) DO UPDATE SET
    tenant_id = EXCLUDED.tenant_id,
    schedule_id = EXCLUDED.schedule_id,
    status = EXCLUDED.status,
    request = EXCLUDED.request,
    records = EXCLUDED.records,
    bytes = EXCLUDED.bytes,
    started_at = EXCLUDED.started_at,
    finished_at = EXCLUDED.finished_at,
    error = EXCLUDED.error,
    updated_at = now();

-- name: DeleteRun :exec
DELETE FROM runs WHERE run_id = @run_id;

-- name: LoadRun :one
SELECT run_id, tenant_id, coalesce(schedule_id, '')::text AS schedule_id, status, request, records, bytes, started_at, finished_at, coalesce(error, '')::text AS error
FROM runs WHERE run_id = @run_id;
