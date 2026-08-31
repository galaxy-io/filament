-- +goose Up
-- The default run listing orders by effective time — started_at, falling back
-- to requested_at, scheduled_at, then created_at — so the 00003 started_at
-- indexes no longer match it. Replace them with expression indexes on the
-- COALESCE the query sorts by; a single DESC index also serves ASC via a
-- backward scan.
DROP INDEX runs_tenant_started_idx;
DROP INDEX runs_pipeline_started_idx;

CREATE INDEX runs_tenant_effective_time_idx
  ON runs (tenant_id, COALESCE(started_at, requested_at, scheduled_at, created_at) DESC, id DESC);

CREATE INDEX runs_pipeline_effective_time_idx
  ON runs (pipeline_id, COALESCE(started_at, requested_at, scheduled_at, created_at) DESC, id DESC);

-- +goose Down
DROP INDEX runs_pipeline_effective_time_idx;
DROP INDEX runs_tenant_effective_time_idx;

CREATE INDEX runs_tenant_started_idx
  ON runs (tenant_id, started_at DESC NULLS FIRST, id DESC);

CREATE INDEX runs_pipeline_started_idx
  ON runs (pipeline_id, started_at DESC NULLS FIRST, id DESC);
