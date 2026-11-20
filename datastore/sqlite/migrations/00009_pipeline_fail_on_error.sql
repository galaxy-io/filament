-- +goose Up
-- Existing pipelines keep stopping at the first resource failure. New
-- pipelines isolate resource failures.
ALTER TABLE pipelines ADD COLUMN fail_on_error INTEGER NOT NULL DEFAULT 0 CHECK (fail_on_error IN (0, 1));
UPDATE pipelines SET fail_on_error = 1;

-- +goose Down
ALTER TABLE pipelines DROP COLUMN fail_on_error;
