-- name: GetStreamEpoch :one
SELECT * FROM stream_epochs WHERE stream_id = sqlc.arg(stream_id) AND tenant_id = sqlc.arg(tenant_id) AND epoch = sqlc.arg(epoch);
-- name: CreateStreamEpoch :one
WITH inserted AS (
 INSERT INTO stream_epochs(stream_id,tenant_id,epoch,attempt_token,certificate,positions)
 VALUES(sqlc.arg(stream_id),sqlc.arg(tenant_id),sqlc.arg(epoch),sqlc.arg(attempt_token),sqlc.arg(certificate),sqlc.arg(positions))
 RETURNING *
), progress AS (
 UPDATE runs SET
  records=runs.records+sqlc.arg(records)::bigint,
  bytes=runs.bytes+sqlc.arg(bytes)::bigint,
  last_committed_at=inserted.committed_at,
  updated_at=inserted.committed_at
 FROM inserted
 WHERE runs.tenant_id=inserted.tenant_id AND runs.id=sqlc.arg(run_id)::uuid
 RETURNING runs.id
)
SELECT inserted.* FROM inserted CROSS JOIN progress;
-- name: AdvanceStreamEpoch :execrows
UPDATE replication_streams SET last_epoch = sqlc.arg(epoch) WHERE id = sqlc.arg(stream_id) AND tenant_id = sqlc.arg(tenant_id) AND last_epoch = sqlc.arg(previous_epoch);
