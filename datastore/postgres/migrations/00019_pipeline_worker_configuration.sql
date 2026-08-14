-- +goose Up
-- Adds the pipeline's default worker configuration, consulted only by the
-- Kubernetes dispatcher. It lives on the pipeline rather than on
-- pipeline_versions because resource_checkpoints are keyed by pipeline version:
-- versioning a resource edit would orphan every cursor on the pipeline and
-- force a re-snapshot. JSONB rather than a column per knob, since the shape
-- grows (cpu/memory today, placement later) and nothing queries by its values.
ALTER TABLE pipelines ADD COLUMN worker_configuration JSONB NOT NULL DEFAULT '{}'::jsonb;

-- +goose Down
ALTER TABLE pipelines DROP COLUMN worker_configuration;
