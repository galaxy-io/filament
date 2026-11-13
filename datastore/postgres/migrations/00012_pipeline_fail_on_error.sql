-- +goose Up
-- Existing pipelines keep stopping at the first resource failure. New
-- pipelines isolate resource failures.
ALTER TABLE pipelines ADD COLUMN fail_on_error BOOLEAN NOT NULL DEFAULT TRUE;
ALTER TABLE pipelines ALTER COLUMN fail_on_error SET DEFAULT FALSE;

-- +goose Down
ALTER TABLE pipelines DROP COLUMN fail_on_error;
