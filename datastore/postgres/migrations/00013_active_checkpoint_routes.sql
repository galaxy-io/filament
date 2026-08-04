-- +goose Up
CREATE UNIQUE INDEX runs_active_checkpoint_route_idx
ON runs (
  (request->>'PipelineID'),
  (request->>'PipelineVersionID'),
  (request->>'CheckpointRoute')
)
WHERE status IN (0, 1, 5)
  AND nullif(request->>'CheckpointRoute', '') IS NOT NULL;

-- +goose Down
DROP INDEX runs_active_checkpoint_route_idx;
