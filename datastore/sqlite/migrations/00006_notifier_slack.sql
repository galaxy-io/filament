-- +goose Up
-- SQLite cannot alter a CHECK constraint, so the table is rebuilt.
CREATE TABLE notifier_rebuild (
  id                   TEXT    PRIMARY KEY,
  tenant_id            TEXT    NOT NULL REFERENCES tenants (id),
  pipeline_id          TEXT    NOT NULL,
  name                 TEXT    NOT NULL,
  notification_type    TEXT    NOT NULL CHECK (notification_type IN ('webhook', 'slack')),
  is_enabled           INTEGER NOT NULL DEFAULT 0,
  events               TEXT    NOT NULL,
  resources            TEXT    NOT NULL DEFAULT '[]',
  config               TEXT    NOT NULL DEFAULT '{}',
  secret_refs          TEXT    NOT NULL DEFAULT '{}',
  is_deleted           INTEGER NOT NULL DEFAULT 0,
  deleted_at           INTEGER,
  created_by_user_id   TEXT,
  updated_by_user_id   TEXT,
  deleted_by_user_id   TEXT,
  created_at           INTEGER NOT NULL,
  updated_at           INTEGER NOT NULL,
  FOREIGN KEY (pipeline_id, tenant_id) REFERENCES pipelines (id, tenant_id)
);

INSERT INTO notifier_rebuild SELECT * FROM notifier;

DROP TABLE notifier;

ALTER TABLE notifier_rebuild RENAME TO notifier;

CREATE INDEX notifier_pipeline_idx ON notifier (tenant_id, pipeline_id, id)
  WHERE is_deleted = 0;

-- +goose Down
CREATE TABLE notifier_rebuild (
  id                   TEXT    PRIMARY KEY,
  tenant_id            TEXT    NOT NULL REFERENCES tenants (id),
  pipeline_id          TEXT    NOT NULL,
  name                 TEXT    NOT NULL,
  notification_type    TEXT    NOT NULL CHECK (notification_type IN ('webhook')),
  is_enabled           INTEGER NOT NULL DEFAULT 0,
  events               TEXT    NOT NULL,
  resources            TEXT    NOT NULL DEFAULT '[]',
  config               TEXT    NOT NULL DEFAULT '{}',
  secret_refs          TEXT    NOT NULL DEFAULT '{}',
  is_deleted           INTEGER NOT NULL DEFAULT 0,
  deleted_at           INTEGER,
  created_by_user_id   TEXT,
  updated_by_user_id   TEXT,
  deleted_by_user_id   TEXT,
  created_at           INTEGER NOT NULL,
  updated_at           INTEGER NOT NULL,
  FOREIGN KEY (pipeline_id, tenant_id) REFERENCES pipelines (id, tenant_id)
);

INSERT INTO notifier_rebuild SELECT * FROM notifier WHERE notification_type = 'webhook';

DROP TABLE notifier;

ALTER TABLE notifier_rebuild RENAME TO notifier;

CREATE INDEX notifier_pipeline_idx ON notifier (tenant_id, pipeline_id, id)
  WHERE is_deleted = 0;
