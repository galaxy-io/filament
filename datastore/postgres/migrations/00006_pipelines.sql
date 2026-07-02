-- +goose Up
CREATE TABLE pipelines (
  pipeline_id TEXT   PRIMARY KEY,
  tenant_id   TEXT   NOT NULL,
  name        TEXT   NOT NULL,
  nodes       JSONB  NOT NULL,
  edges       JSONB  NOT NULL,
  version     BIGINT NOT NULL DEFAULT 1,
  created_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at  TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX pipelines_tenant_idx ON pipelines (tenant_id);

-- +goose Down
DROP TABLE pipelines;
