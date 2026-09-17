-- name: GetReplicationStreamPipelineID :one
SELECT pipeline_id FROM replication_streams WHERE id = sqlc.arg(stream_id) AND tenant_id = sqlc.arg(tenant_id) AND generation = sqlc.arg(generation);
-- name: LockReplicationStreamGeneration :one
SELECT * FROM replication_streams WHERE id = sqlc.arg(stream_id) AND tenant_id = sqlc.arg(tenant_id) AND generation = sqlc.arg(generation) FOR UPDATE;
-- name: LoadRunRequest :one
SELECT r.request FROM runs r JOIN pipeline_versions v ON v.id = r.pipeline_version_id
WHERE r.id = sqlc.arg(run_id) AND r.tenant_id = sqlc.arg(tenant_id) AND r.pipeline_id = sqlc.arg(pipeline_id)
AND r.pipeline_version_id = sqlc.arg(pipeline_version_id) AND v.tenant_id = sqlc.arg(tenant_id);
-- name: InitializeStreamExecution :exec
UPDATE replication_streams SET current_run_id = sqlc.arg(run_id), desired_state = 'enabled',
 desired_revision = 1, run_spec = sqlc.arg(run_spec), updated_at = clock_timestamp()
WHERE id = sqlc.arg(stream_id) AND tenant_id = sqlc.arg(tenant_id) AND current_run_id IS NULL;
-- name: GetStreamExecution :one
SELECT coalesce(s.current_run_id::text, '')::text AS run_id,
 coalesce(s.run_spec->>'PipelineVersionID', '')::text AS pipeline_version_id,
 s.desired_state, s.desired_revision AS revision, s.run_spec, s.last_epoch
FROM replication_streams s
WHERE s.id = sqlc.arg(stream_id) AND s.tenant_id = sqlc.arg(tenant_id) AND s.current_run_id IS NOT NULL;
-- name: ChangeStreamDesiredState :execrows
UPDATE replication_streams SET desired_state = sqlc.arg(desired_state), desired_revision = desired_revision + 1, updated_at = clock_timestamp()
WHERE id = sqlc.arg(stream_id) AND tenant_id = sqlc.arg(tenant_id) AND desired_revision = sqlc.arg(revision) AND current_run_id IS NOT NULL;
-- name: GetStreamAttemptByExecutionID :one
SELECT * FROM stream_attempts WHERE tenant_id = sqlc.arg(tenant_id) AND execution_id = sqlc.arg(execution_id);
-- name: GetLatestRouteAttempt :one
SELECT a.* FROM stream_attempts a JOIN replication_streams s ON s.id = a.stream_id
WHERE s.pipeline_id = sqlc.arg(pipeline_id) AND s.route_key = sqlc.arg(route) AND s.tenant_id = sqlc.arg(tenant_id)
ORDER BY a.token DESC LIMIT 1;
-- name: GetLatestStreamAttempt :one
SELECT * FROM stream_attempts WHERE stream_id = sqlc.arg(stream_id) AND tenant_id = sqlc.arg(tenant_id) ORDER BY token DESC LIMIT 1;
-- name: CreateStreamAttempt :one
INSERT INTO stream_attempts(stream_id,tenant_id,execution_id,desired_revision,request_ttl_us,expires_at,run_spec)
VALUES(sqlc.arg(stream_id),sqlc.arg(tenant_id),sqlc.arg(execution_id),sqlc.arg(desired_revision),sqlc.arg(ttl_us),clock_timestamp() + sqlc.arg(ttl_us)::bigint * interval '1 microsecond', (SELECT run_spec FROM replication_streams WHERE id=sqlc.arg(stream_id))) RETURNING *;
-- name: Now :one
SELECT clock_timestamp()::timestamptz;
-- name: RenewStreamAttempt :execrows
UPDATE stream_attempts SET expires_at = GREATEST(expires_at, clock_timestamp() + sqlc.arg(ttl_us)::bigint * interval '1 microsecond')
WHERE token = sqlc.arg(token) AND tenant_id = sqlc.arg(tenant_id) AND ended_at IS NULL AND expires_at > clock_timestamp();
-- name: EndStreamAttempt :execrows
UPDATE stream_attempts SET ended_at = clock_timestamp(), termination = sqlc.arg(termination), reason = sqlc.arg(reason)
WHERE token = sqlc.arg(token) AND tenant_id = sqlc.arg(tenant_id) AND ended_at IS NULL AND (NOT sqlc.arg(require_live)::boolean OR expires_at > clock_timestamp());
-- name: ListReconcilableStreamIDs :many
SELECT id FROM replication_streams
WHERE tenant_id = sqlc.arg(tenant_id) AND status = 0 AND current_run_id IS NOT NULL AND id::text > sqlc.arg(after_id)::text
ORDER BY id::text LIMIT sqlc.arg(page_limit);
-- name: LatestStreamPositions :one
SELECT e.positions FROM replication_streams s JOIN stream_epochs e ON e.stream_id = s.id AND e.epoch = s.last_epoch
WHERE s.id = sqlc.arg(stream_id) AND s.tenant_id = sqlc.arg(tenant_id);

-- name: ClaimStreamAttempt :execrows
UPDATE stream_attempts a SET claimed_at=clock_timestamp()
FROM replication_streams s
WHERE a.stream_id=s.id AND a.token=sqlc.arg(token) AND a.tenant_id=sqlc.arg(tenant_id)
AND a.execution_id=sqlc.arg(execution_id) AND s.current_run_id=sqlc.arg(run_id)
AND s.status=0 AND s.desired_state='enabled' AND a.claimed_at IS NULL
AND a.ended_at IS NULL AND a.expires_at>clock_timestamp();

-- name: RetireUnclaimedAttempt :execrows
UPDATE stream_attempts a SET ended_at=clock_timestamp(),termination='clean',reason='dispatch retired before worker claim'
WHERE a.token=sqlc.arg(token) AND a.tenant_id=sqlc.arg(tenant_id) AND a.claimed_at IS NULL
AND a.ended_at IS NULL AND (a.expires_at<=clock_timestamp() OR EXISTS (SELECT 1 FROM replication_streams s WHERE s.id=a.stream_id AND s.desired_state<>'enabled'));

-- name: PendingContinuousRuns :many
SELECT r.id,r.tenant_id FROM runs r JOIN replication_streams s ON s.current_run_id=r.id
WHERE s.status=0 AND r.ended_at IS NULL AND r.id::text>sqlc.arg(after_id)::text
ORDER BY r.id::text LIMIT sqlc.arg(page_limit);

-- name: ActivateMessageResource :exec
INSERT INTO replication_stream_resources(replication_stream_id,tenant_id,resource_name,status,bootstrap_mode,bootstrap_config)
VALUES(sqlc.arg(stream_id),sqlc.arg(tenant_id),sqlc.arg(resource_name),2,'none','{}')
ON CONFLICT (replication_stream_id,resource_name) DO NOTHING;

-- name: ReplaceStoppedActivation :execrows
UPDATE replication_streams SET current_run_id=sqlc.arg(run_id),run_spec=sqlc.arg(run_spec),desired_state='enabled',
 desired_revision=desired_revision+1,updated_at=clock_timestamp()
WHERE id=sqlc.arg(stream_id) AND tenant_id=sqlc.arg(tenant_id) AND desired_state='stopped';

-- name: FinishStoppedContinuousRun :exec
UPDATE runs r SET status=2,ended_at=clock_timestamp(),updated_at=clock_timestamp()
FROM replication_streams s WHERE s.current_run_id=r.id AND s.id=sqlc.arg(stream_id) AND s.tenant_id=sqlc.arg(tenant_id)
AND r.ended_at IS NULL AND s.desired_state='stopped' AND NOT EXISTS (SELECT 1 FROM stream_attempts a WHERE a.stream_id=s.id AND (a.ended_at IS NULL OR a.termination<>'clean'));

-- name: MarkContinuousRunStarted :exec
UPDATE runs SET status=1,started_at=coalesce(started_at,clock_timestamp()),updated_at=clock_timestamp()
WHERE id=sqlc.arg(run_id) AND tenant_id=sqlc.arg(tenant_id) AND ended_at IS NULL;

-- name: SyncContinuousRunPhase :exec
UPDATE runs r SET status=CASE
 WHEN EXISTS (SELECT 1 FROM stream_attempts a WHERE a.stream_id=s.id AND a.ended_at IS NULL AND a.claimed_at IS NOT NULL) THEN 1
 WHEN s.desired_state='paused' THEN 5
 ELSE 0 END,updated_at=clock_timestamp()
FROM replication_streams s WHERE s.current_run_id=r.id AND s.id=sqlc.arg(stream_id) AND s.tenant_id=sqlc.arg(tenant_id)
AND r.ended_at IS NULL AND s.desired_state<>'stopped';
