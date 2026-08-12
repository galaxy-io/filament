-- +goose Up
ALTER TABLE runs
  ADD COLUMN cpu_seconds       DOUBLE PRECISION NOT NULL DEFAULT 0,
  ADD COLUMN memory_peak_bytes BIGINT           NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE runs
  DROP COLUMN cpu_seconds,
  DROP COLUMN memory_peak_bytes;
