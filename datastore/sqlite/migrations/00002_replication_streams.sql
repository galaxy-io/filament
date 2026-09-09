-- +goose Up
-- Composite foreign keys require matching unique keys in SQLite.
CREATE UNIQUE INDEX pipelines_id_tenant_idx ON pipelines (id, tenant_id);
CREATE UNIQUE INDEX pipeline_versions_pipeline_id_id_idx ON pipeline_versions (pipeline_id, id);

-- Streams preserve consumer identity across compatible pipeline versions.
CREATE TABLE replication_streams (
  id                               TEXT    PRIMARY KEY,
  tenant_id                        TEXT    NOT NULL REFERENCES tenants (id),
  pipeline_id                      TEXT    NOT NULL,
  route_key                        TEXT    NOT NULL,
  generation                       INTEGER NOT NULL DEFAULT 1 CHECK (generation > 0),
  source_connection_id             TEXT    NOT NULL REFERENCES connections (id),
  sink_connection_id               TEXT    NOT NULL REFERENCES connections (id),
  consumer_name                    TEXT    NOT NULL,
  consumer_config                  TEXT    NOT NULL DEFAULT '{}',
  continuity_fingerprint           TEXT    NOT NULL,
  -- 0=active, 1=retired, 2=error; kept numeric for protobuf mapping.
  status                           INTEGER NOT NULL DEFAULT 0
                                         CHECK (status IN (0, 1, 2)),
  created_from_pipeline_version_id TEXT    NOT NULL,
  error                            TEXT,
  retired_at                       INTEGER,
  created_by_user_id               TEXT,
  updated_by_user_id               TEXT,
  deleted_by_user_id               TEXT,
  created_at                       INTEGER NOT NULL,
  updated_at                       INTEGER NOT NULL,
  FOREIGN KEY (pipeline_id, tenant_id)
    REFERENCES pipelines (id, tenant_id),
  FOREIGN KEY (pipeline_id, created_from_pipeline_version_id)
    REFERENCES pipeline_versions (pipeline_id, id),
  UNIQUE (pipeline_id, route_key, generation),
  UNIQUE (source_connection_id, consumer_name),
  UNIQUE (id, tenant_id)
);

-- Only one active stream can own a pipeline route.
CREATE UNIQUE INDEX replication_streams_active_route_idx
  ON replication_streams (pipeline_id, route_key)
  WHERE status = 0;

CREATE INDEX replication_streams_source_connection_idx
  ON replication_streams (source_connection_id);

CREATE INDEX replication_streams_sink_connection_idx
  ON replication_streams (sink_connection_id);

-- Track resource membership before a checkpoint exists and after retirement.
CREATE TABLE replication_stream_resources (
  id                               TEXT    PRIMARY KEY,
  replication_stream_id            TEXT    NOT NULL,
  tenant_id                        TEXT    NOT NULL REFERENCES tenants (id),
  resource_name                    TEXT    NOT NULL,
  -- 0=pending, 1=bootstrapping, 2=active, 3=retired, 4=error.
  status                           INTEGER NOT NULL DEFAULT 0
                                         CHECK (status IN (0, 1, 2, 3, 4)),
  bootstrap_mode                   TEXT    NOT NULL,
  bootstrap_config                 TEXT    NOT NULL DEFAULT '{}',
  schema_fingerprint               TEXT,
  bootstrap_run_id                 TEXT    REFERENCES runs (id) ON DELETE SET NULL,
  bootstrap_started_at             INTEGER,
  activated_at                     INTEGER,
  retired_at                       INTEGER,
  error                            TEXT,
  created_by_user_id               TEXT,
  updated_by_user_id               TEXT,
  deleted_by_user_id               TEXT,
  created_at                       INTEGER NOT NULL,
  updated_at                       INTEGER NOT NULL,
  UNIQUE (replication_stream_id, resource_name),
  FOREIGN KEY (replication_stream_id, tenant_id)
    REFERENCES replication_streams (id, tenant_id)
    ON DELETE CASCADE
);

CREATE INDEX replication_stream_resources_status_idx
  ON replication_stream_resources (replication_stream_id, status);

-- Stream checkpoints are shared across compatible pipeline versions.
ALTER TABLE pipeline_resource_checkpoints
  ADD COLUMN replication_stream_resource_id TEXT
    REFERENCES replication_stream_resources (id) ON DELETE CASCADE;

CREATE UNIQUE INDEX pipeline_resource_checkpoints_stream_resource_idx
  ON pipeline_resource_checkpoints (replication_stream_resource_id)
  WHERE replication_stream_resource_id IS NOT NULL;

-- Prevent concurrent stream runs on the same route, even across versions.
CREATE UNIQUE INDEX runs_active_replication_route_idx
  ON runs (pipeline_id, json_extract(request, '$.CheckpointRoute'))
  WHERE status IN (0, 1, 5)
    AND nullif(json_extract(request, '$.ReplicationStreamID'), '') IS NOT NULL;

-- +goose Down
DROP INDEX runs_active_replication_route_idx;
DROP INDEX pipeline_resource_checkpoints_stream_resource_idx;

ALTER TABLE pipeline_resource_checkpoints
  DROP COLUMN replication_stream_resource_id;

DROP INDEX replication_stream_resources_status_idx;
DROP TABLE replication_stream_resources;

DROP INDEX replication_streams_sink_connection_idx;
DROP INDEX replication_streams_source_connection_idx;
DROP INDEX replication_streams_active_route_idx;
DROP TABLE replication_streams;

DROP INDEX pipeline_versions_pipeline_id_id_idx;
DROP INDEX pipelines_id_tenant_idx;
