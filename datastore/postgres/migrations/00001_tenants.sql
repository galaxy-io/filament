-- +goose Up
CREATE TABLE tenants (
  tenant_id  TEXT        PRIMARY KEY,
  name       TEXT,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- +goose Down
DROP TABLE tenants;
