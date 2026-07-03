-- +goose Up
CREATE TABLE checkpoints (
  run_id        TEXT   NOT NULL REFERENCES runs (run_id) ON DELETE CASCADE,
  resource_name TEXT   NOT NULL,
  cursor        JSONB  NOT NULL,
  updated_at    TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (run_id, resource_name)
);

-- +goose Down
DROP TABLE checkpoints;
