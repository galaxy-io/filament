-- +goose Up
-- Idempotency is enforced by the run id itself: runs.IDFor derives it from
-- (tenant, IdempotencyKey), so the primary key already guarantees one run per
-- key and this index was a redundant second guard with no memory-store
-- counterpart.
DROP INDEX runs_idempotency_idx;

-- +goose Down
CREATE UNIQUE INDEX runs_idempotency_idx ON runs (tenant_id, (request->>'IdempotencyKey'))
  WHERE nullif(request->>'IdempotencyKey', '') IS NOT NULL;
