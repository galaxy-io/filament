-- +goose Up
CREATE UNIQUE INDEX connections_id_tenant_unique ON connections (id, tenant_id);

CREATE TABLE webhook_endpoints (
  source_connection_id       TEXT    PRIMARY KEY,
  tenant_id                  TEXT    NOT NULL,
  pipeline_id                TEXT,
  route_key                  TEXT,
  config_revision            INTEGER NOT NULL DEFAULT 1 CHECK (config_revision > 0),
  enabled                    INTEGER NOT NULL DEFAULT 0 CHECK (enabled IN (0, 1)),
  verifier                   TEXT    NOT NULL DEFAULT 'generic_hmac_sha256' CHECK (verifier <> ''),
  secret_refs                TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(secret_refs) AND json_type(secret_refs) = 'array'),
  previous_key_expires_at    INTEGER,
  resources                  TEXT    NOT NULL DEFAULT '[]' CHECK (json_valid(resources) AND json_type(resources) = 'array'),
  success_status             INTEGER NOT NULL DEFAULT 202 CHECK (success_status IN (200, 202, 204)),
  max_body_bytes             INTEGER NOT NULL DEFAULT 1048576 CHECK (max_body_bytes > 0 AND max_body_bytes <= 16777216),
  created_at                 INTEGER NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),
  updated_at                 INTEGER NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),
  FOREIGN KEY (source_connection_id, tenant_id) REFERENCES connections (id, tenant_id),
  FOREIGN KEY (pipeline_id, tenant_id) REFERENCES pipelines (id, tenant_id),
  CHECK ((pipeline_id IS NULL) = (route_key IS NULL)),
  CHECK (route_key IS NULL OR length(route_key) > 0),
  CHECK (json_array_length(secret_refs) <= 2),
  CHECK (enabled = 0 OR (pipeline_id IS NOT NULL AND json_array_length(secret_refs) >= 1
    AND json_array_length(resources) >= 1))
);

CREATE INDEX webhook_endpoints_tenant_idx ON webhook_endpoints (tenant_id, source_connection_id);

-- +goose Down
DROP INDEX webhook_endpoints_tenant_idx;
DROP TABLE webhook_endpoints;
DROP INDEX connections_id_tenant_unique;
