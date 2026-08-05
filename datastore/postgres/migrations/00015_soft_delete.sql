-- +goose Up
ALTER TABLE connections ADD COLUMN is_deleted BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE connections ADD COLUMN deleted_at TIMESTAMPTZ;
ALTER TABLE pipelines   ADD COLUMN is_deleted BOOLEAN NOT NULL DEFAULT false;
ALTER TABLE pipelines   ADD COLUMN deleted_at TIMESTAMPTZ;

DROP INDEX connections_tenant_name_kind_idx;
CREATE UNIQUE INDEX connections_tenant_name_kind_idx
  ON connections (tenant_id, kind, name) WHERE NOT is_deleted;

-- +goose Down
DROP INDEX connections_tenant_name_kind_idx;
CREATE UNIQUE INDEX connections_tenant_name_kind_idx
  ON connections (tenant_id, kind, name);

ALTER TABLE connections DROP COLUMN is_deleted, DROP COLUMN deleted_at;
ALTER TABLE pipelines   DROP COLUMN is_deleted, DROP COLUMN deleted_at;
