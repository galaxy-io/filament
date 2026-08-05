-- +goose Up
-- Adds the latest run's finish time to the pipeline run summary so lists can
-- show duration without joining runs. Backfilled from each pipeline's most
-- recent run.
ALTER TABLE pipelines ADD COLUMN last_run_ended_at TIMESTAMPTZ;

UPDATE pipelines p SET last_run_ended_at = r.finished_at
FROM (
  SELECT DISTINCT ON (request->>'PipelineID') request->>'PipelineID' AS run_pipeline_id, finished_at
  FROM runs WHERE request->>'PipelineID' IS NOT NULL
  ORDER BY request->>'PipelineID', started_at DESC
) r
WHERE p.pipeline_id = r.run_pipeline_id;

-- +goose Down
ALTER TABLE pipelines DROP COLUMN last_run_ended_at;
