-- +goose Up
ALTER TABLE stream_attempts ADD COLUMN dispatch_version TEXT NOT NULL DEFAULT '';
ALTER TABLE stream_attempts ADD COLUMN yield_requested_at INTEGER;

-- +goose Down
ALTER TABLE stream_attempts DROP COLUMN yield_requested_at;
ALTER TABLE stream_attempts DROP COLUMN dispatch_version;
