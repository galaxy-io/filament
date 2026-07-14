-- +goose Up
CREATE TYPE connection_kind AS ENUM ('source', 'sink');

CREATE TABLE connections (
  connection_id TEXT     PRIMARY KEY,
  tenant_id     TEXT     NOT NULL REFERENCES tenants (tenant_id),
  kind          connection_kind NOT NULL,     -- source | sink (ConnectorKind)
  name          TEXT     NOT NULL,
  provider      TEXT     NOT NULL,            -- e.g. "mysql", "postgres", "object"
  config        JSONB    NOT NULL DEFAULT '{}',  -- connection-scoped fields only
  secret_refs   JSONB    NOT NULL DEFAULT '{}',
  version       BIGINT   NOT NULL DEFAULT 1,  -- optimistic lock, mirrors pipelines
  created_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX connections_tenant_kind_idx ON connections (tenant_id, kind);
CREATE UNIQUE INDEX connections_tenant_name_kind_idx ON connections (tenant_id, kind, name);

-- +goose Down
DROP TABLE connections;
DROP TYPE connection_kind;
