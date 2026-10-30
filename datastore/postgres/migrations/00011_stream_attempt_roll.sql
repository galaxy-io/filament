-- +goose Up
ALTER TABLE stream_attempts
  ADD COLUMN dispatch_version TEXT NOT NULL DEFAULT '',
  ADD COLUMN yield_requested_at TIMESTAMPTZ;

-- +goose Down
ALTER TABLE stream_attempts
  DROP COLUMN yield_requested_at,
  DROP COLUMN dispatch_version;
