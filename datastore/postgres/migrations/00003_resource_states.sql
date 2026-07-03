-- +goose Up
CREATE TABLE resource_states (
  run_id        TEXT     NOT NULL REFERENCES runs (run_id) ON DELETE CASCADE,
  resource_name TEXT     NOT NULL,
  tenant_id     TEXT     NOT NULL REFERENCES tenants (tenant_id),
  enabled       BOOLEAN  NOT NULL DEFAULT true,
  status        SMALLINT NOT NULL,
  records       BIGINT   NOT NULL DEFAULT 0,
  bytes         BIGINT   NOT NULL DEFAULT 0,
  error         TEXT,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, resource_name)
);

-- +goose Down
DROP TABLE resource_states;
