-- +goose Up
ALTER TABLE tenants
  ADD COLUMN external_id TEXT UNIQUE;

-- +goose Down
ALTER TABLE tenants
  DROP COLUMN external_id;
