-- +goose Up
CREATE TYPE notification_type AS ENUM ('webhook');

CREATE TABLE notifier (
  id                   UUID              PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id            UUID              NOT NULL REFERENCES tenants (id),
  pipeline_id          UUID              NOT NULL,
  name                 TEXT              NOT NULL,
  notification_type    notification_type NOT NULL,
  enabled              BOOLEAN           NOT NULL DEFAULT false,
  events               JSONB             NOT NULL,
  resources            JSONB             NOT NULL DEFAULT '[]',
  config               JSONB             NOT NULL DEFAULT '{}',
  secret_refs          JSONB             NOT NULL DEFAULT '{}',
  version              BIGINT            NOT NULL DEFAULT 1,
  is_deleted           BOOLEAN           NOT NULL DEFAULT false,
  deleted_at           TIMESTAMPTZ,
  created_by_user_id   UUID              REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id   UUID              REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id   UUID              REFERENCES users (id) ON DELETE SET NULL,
  created_at           TIMESTAMPTZ        NOT NULL DEFAULT now(),
  updated_at           TIMESTAMPTZ        NOT NULL DEFAULT now(),
  FOREIGN KEY (pipeline_id, tenant_id) REFERENCES pipelines (id, tenant_id)
);

CREATE INDEX notifier_pipeline_idx ON notifier (tenant_id, pipeline_id, id)
  WHERE NOT is_deleted;

-- +goose Down
DROP TABLE notifier;
DROP TYPE notification_type;
