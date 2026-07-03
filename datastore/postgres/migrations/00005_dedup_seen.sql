-- +goose Up
CREATE TABLE dedup_seen (
  tenant_id TEXT   NOT NULL REFERENCES tenants (tenant_id),
  run_id    TEXT   NOT NULL REFERENCES runs (run_id) ON DELETE CASCADE,
  last_seq  BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, run_id)
);

-- +goose Down
DROP TABLE dedup_seen;
