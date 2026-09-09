-- +goose Up
CREATE TABLE notifier (
  id                   TEXT    PRIMARY KEY,
  tenant_id            TEXT    NOT NULL REFERENCES tenants (id),
  pipeline_id          TEXT    NOT NULL,
  name                 TEXT    NOT NULL,
  notification_type    TEXT    NOT NULL CHECK (notification_type IN ('webhook')),
  enabled              INTEGER NOT NULL DEFAULT 0,
  events               TEXT    NOT NULL,
  resources            TEXT    NOT NULL DEFAULT '[]',
  config               TEXT    NOT NULL DEFAULT '{}',
  secret_refs          TEXT    NOT NULL DEFAULT '{}',
  version              INTEGER NOT NULL DEFAULT 1,
  is_deleted           INTEGER NOT NULL DEFAULT 0,
  deleted_at           INTEGER,
  created_by_user_id   TEXT,
  updated_by_user_id   TEXT,
  deleted_by_user_id   TEXT,
  created_at           INTEGER NOT NULL,
  updated_at           INTEGER NOT NULL,
  FOREIGN KEY (pipeline_id, tenant_id) REFERENCES pipelines (id, tenant_id)
);

CREATE INDEX notifier_pipeline_idx ON notifier (tenant_id, pipeline_id, id)
  WHERE is_deleted = 0;

-- +goose Down
DROP TABLE notifier;
