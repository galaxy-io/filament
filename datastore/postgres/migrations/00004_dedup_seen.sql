-- +goose Up
CREATE TABLE dedup_seen (
  tenant_id TEXT   NOT NULL,
  run_id    TEXT   NOT NULL,
  last_seq  BIGINT NOT NULL,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (tenant_id, run_id)
);

-- +goose Down
DROP TABLE dedup_seen;
