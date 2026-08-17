-- name: AdvanceDedupSeen :one
-- Advances the (tenant, run) high-water mark only when @seq increases.
INSERT INTO run_dedup_seen (tenant_id, run_id, last_seq, updated_at)
VALUES (@tenant_id, @run_id, @seq, now())
ON CONFLICT (tenant_id, run_id) DO UPDATE
  SET last_seq = EXCLUDED.last_seq, updated_at = now()
  WHERE run_dedup_seen.last_seq < EXCLUDED.last_seq
RETURNING last_seq;
