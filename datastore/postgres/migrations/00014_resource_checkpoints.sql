-- +goose Up
CREATE TABLE resource_checkpoints (
  pipeline_id        TEXT        NOT NULL,
  pipeline_version   BIGINT      NOT NULL,
  route_key          TEXT        NOT NULL,
  resource_name      TEXT        NOT NULL,
  cursor             JSONB       NOT NULL,
  last_run_id        TEXT        NOT NULL,
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (pipeline_id, pipeline_version, route_key, resource_name),
  FOREIGN KEY (pipeline_id, pipeline_version)
    REFERENCES pipeline_versions (pipeline_id, version) ON DELETE CASCADE
);

CREATE INDEX resource_checkpoints_last_run_idx ON resource_checkpoints (last_run_id);

-- +goose Down
DROP TABLE resource_checkpoints;
