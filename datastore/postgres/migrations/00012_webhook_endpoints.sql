-- +goose Up
ALTER TABLE connections ADD CONSTRAINT connections_id_tenant_unique UNIQUE (id, tenant_id);

CREATE TABLE webhook_endpoints (
  source_connection_id UUID PRIMARY KEY,
  tenant_id UUID NOT NULL,
  pipeline_id UUID,
  route_key TEXT,
  config_revision BIGINT NOT NULL DEFAULT 1 CHECK (config_revision > 0),
  enabled BOOLEAN NOT NULL DEFAULT false,
  verifier TEXT NOT NULL DEFAULT 'generic_hmac_sha256' CHECK (verifier <> ''),
  secret_refs TEXT[] NOT NULL DEFAULT '{}',
  previous_key_expires_at TIMESTAMPTZ,
  resources TEXT[] NOT NULL DEFAULT '{}',
  success_status SMALLINT NOT NULL DEFAULT 202 CHECK (success_status IN (200, 202, 204)),
  max_body_bytes BIGINT NOT NULL DEFAULT 1048576
    CHECK (max_body_bytes > 0 AND max_body_bytes <= 16777216),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (source_connection_id, tenant_id) REFERENCES connections (id, tenant_id),
  FOREIGN KEY (pipeline_id, tenant_id) REFERENCES pipelines (id, tenant_id),
  CHECK ((pipeline_id IS NULL) = (route_key IS NULL)),
  CHECK (route_key IS NULL OR length(route_key) > 0),
  CHECK (cardinality(secret_refs) <= 2),
  CHECK (NOT enabled OR (pipeline_id IS NOT NULL AND cardinality(secret_refs) >= 1
    AND cardinality(resources) >= 1))
);

CREATE INDEX webhook_endpoints_tenant_idx ON webhook_endpoints (tenant_id, source_connection_id);

-- +goose Down
DROP TABLE webhook_endpoints;
ALTER TABLE connections DROP CONSTRAINT connections_id_tenant_unique;
