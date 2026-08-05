-- +goose Up
-- Promotes pipeline_id/pipeline_version_id alongside the existing
-- source_provider/ingestion_type generated columns (00002_runs.sql), so
-- metrics queries can group/filter on them without an on the fly JSON path
-- lookup. GENERATED ... STORED backfills every existing row automatically —
-- no separate backfill step.
ALTER TABLE runs ADD COLUMN pipeline_id TEXT GENERATED ALWAYS AS (request->>'PipelineID') STORED;
ALTER TABLE runs ADD COLUMN pipeline_version_id BIGINT GENERATED ALWAYS AS ((request->>'PipelineVersionID')::bigint) STORED;

CREATE INDEX runs_metrics_idx ON runs (tenant_id, pipeline_id, pipeline_version_id, status, started_at);

-- +goose Down
DROP INDEX runs_metrics_idx;
ALTER TABLE runs DROP COLUMN pipeline_version_id;
ALTER TABLE runs DROP COLUMN pipeline_id;
