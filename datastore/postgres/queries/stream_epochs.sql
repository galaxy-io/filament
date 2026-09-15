-- name: GetStreamEpoch :one
SELECT * FROM stream_epochs WHERE stream_id = sqlc.arg(stream_id) AND tenant_id = sqlc.arg(tenant_id) AND epoch = sqlc.arg(epoch);
-- name: CreateStreamEpoch :one
INSERT INTO stream_epochs(stream_id,tenant_id,epoch,attempt_token,certificate,positions)
VALUES(sqlc.arg(stream_id),sqlc.arg(tenant_id),sqlc.arg(epoch),sqlc.arg(attempt_token),sqlc.arg(certificate),sqlc.arg(positions)) RETURNING *;
-- name: AdvanceStreamEpoch :execrows
UPDATE replication_streams SET last_epoch = sqlc.arg(epoch) WHERE id = sqlc.arg(stream_id) AND tenant_id = sqlc.arg(tenant_id) AND last_epoch = sqlc.arg(previous_epoch);
