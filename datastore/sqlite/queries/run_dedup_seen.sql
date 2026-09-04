-- name: AdvanceDedupSeen :one
-- Advances the (tenant, run) high-water mark only when @seq increases.
INSERT INTO run_dedup_seen (tenant_id, run_id, last_seq, created_at, updated_at)
VALUES (@tenant_id, @run_id, @seq, @created_at, @updated_at)
ON CONFLICT (tenant_id, run_id) DO UPDATE
  SET last_seq = excluded.last_seq, updated_at = excluded.updated_at
  WHERE run_dedup_seen.last_seq < excluded.last_seq
RETURNING last_seq;
