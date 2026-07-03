-- name: AdvanceDedupSeen :one
-- Advances the (tenant, run) high-water mark to @seq and returns the new
-- last_seq — but only if @seq is strictly greater than what's stored (or no
-- row exists yet). When @seq is <= the stored value, the WHERE guard skips
-- the update and the query returns zero rows: the caller reads that as
-- "already seen, skip" without a separate SELECT.
INSERT INTO dedup_seen (tenant_id, run_id, last_seq, updated_at)
VALUES (@tenant_id, @run_id, @seq, now())
ON CONFLICT (tenant_id, run_id) DO UPDATE
  SET last_seq = EXCLUDED.last_seq, updated_at = now()
  WHERE dedup_seen.last_seq < EXCLUDED.last_seq
RETURNING last_seq;
