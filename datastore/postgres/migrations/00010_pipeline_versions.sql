-- +goose Up
CREATE TABLE pipeline_versions (
  pipeline_id TEXT NOT NULL REFERENCES pipelines (pipeline_id) ON DELETE CASCADE,
  version BIGINT NOT NULL,
  nodes JSONB NOT NULL,
  edges JSONB NOT NULL,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  PRIMARY KEY (pipeline_id, version)
);

INSERT INTO pipeline_versions (pipeline_id, version, nodes, edges, created_at)
SELECT pipeline_id, version, nodes, edges, created_at FROM pipelines;

ALTER TABLE pipelines ADD COLUMN description TEXT NOT NULL DEFAULT '';
ALTER TABLE pipelines ADD COLUMN current_version_id BIGINT NOT NULL DEFAULT 1;
ALTER TABLE pipelines ADD COLUMN last_run_version_id BIGINT NOT NULL DEFAULT 0;
ALTER TABLE pipelines ADD COLUMN last_run_at TIMESTAMPTZ;
ALTER TABLE pipelines ADD COLUMN last_run_status SMALLINT NOT NULL DEFAULT 0;
ALTER TABLE pipelines ADD COLUMN last_run_bytes BIGINT NOT NULL DEFAULT 0;
ALTER TABLE pipelines DROP COLUMN nodes;
ALTER TABLE pipelines DROP COLUMN edges;
ALTER TABLE pipelines DROP COLUMN version;

-- +goose Down
ALTER TABLE pipelines ADD COLUMN nodes JSONB NOT NULL DEFAULT '[]';
ALTER TABLE pipelines ADD COLUMN edges JSONB NOT NULL DEFAULT '[]';
ALTER TABLE pipelines ADD COLUMN version BIGINT NOT NULL DEFAULT 1;
UPDATE pipelines p SET nodes = v.nodes, edges = v.edges, version = v.version
FROM pipeline_versions v WHERE v.pipeline_id = p.pipeline_id AND v.version = p.current_version_id;
ALTER TABLE pipelines DROP COLUMN description;
ALTER TABLE pipelines DROP COLUMN current_version_id;
ALTER TABLE pipelines DROP COLUMN last_run_version_id;
ALTER TABLE pipelines DROP COLUMN last_run_at;
ALTER TABLE pipelines DROP COLUMN last_run_status;
ALTER TABLE pipelines DROP COLUMN last_run_bytes;
DROP TABLE pipeline_versions;
