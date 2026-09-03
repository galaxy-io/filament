-- +goose Up
-- A replication stream is the durable identity of one independently advancing
-- source-to-sink consumer. Pipeline versions may reuse the stream when an edit
-- preserves continuity; incompatible edits create the next generation.
CREATE TABLE replication_streams (
  id                               UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  tenant_id                        UUID        NOT NULL REFERENCES tenants (id),
  pipeline_id                      UUID        NOT NULL,
  route_key                        TEXT        NOT NULL,
  generation                       BIGINT      NOT NULL DEFAULT 1 CHECK (generation > 0),
  source_connection_id             UUID        NOT NULL REFERENCES connections (id),
  sink_connection_id               UUID        NOT NULL REFERENCES connections (id),
  consumer_name                    TEXT        NOT NULL,
  consumer_config                  JSONB       NOT NULL DEFAULT '{}',
  continuity_fingerprint           TEXT        NOT NULL,
  -- 0=active, 1=retired, 2=error; kept numeric for protobuf mapping.
  status                           SMALLINT    NOT NULL DEFAULT 0
                                             CHECK (status IN (0, 1, 2)),
  created_from_pipeline_version_id UUID        NOT NULL,
  error                            TEXT,
  retired_at                       TIMESTAMPTZ,
  created_by_user_id               UUID        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id               UUID        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id               UUID        REFERENCES users (id) ON DELETE SET NULL,
  created_at                       TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at                       TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (pipeline_id, tenant_id)
    REFERENCES pipelines (id, tenant_id),
  FOREIGN KEY (pipeline_id, created_from_pipeline_version_id)
    REFERENCES pipeline_versions (pipeline_id, id),
  UNIQUE (pipeline_id, route_key, generation),
  UNIQUE (source_connection_id, consumer_name),
  UNIQUE (id, tenant_id)
);

-- One current stream owns a pipeline route. Retired/error generations remain
-- available for audit without blocking a replacement generation.
CREATE UNIQUE INDEX replication_streams_active_route_idx
  ON replication_streams (pipeline_id, route_key)
  WHERE status = 0;

CREATE INDEX replication_streams_source_connection_idx
  ON replication_streams (source_connection_id);

CREATE INDEX replication_streams_sink_connection_idx
  ON replication_streams (sink_connection_id);

-- Resource membership is distinct from checkpoint presence: a newly selected
-- table/topic exists as pending before it owns a cursor, and a retired resource
-- keeps its history without participating in the stream checkpoint floor.
CREATE TABLE replication_stream_resources (
  id                    UUID        PRIMARY KEY DEFAULT gen_random_uuid(),
  replication_stream_id UUID        NOT NULL,
  tenant_id              UUID        NOT NULL REFERENCES tenants (id),
  resource_name          TEXT        NOT NULL,
  -- 0=pending, 1=bootstrapping, 2=active, 3=retired, 4=error.
  status                 SMALLINT    NOT NULL DEFAULT 0
                                   CHECK (status IN (0, 1, 2, 3, 4)),
  bootstrap_mode         TEXT        NOT NULL,
  bootstrap_config       JSONB       NOT NULL DEFAULT '{}',
  schema_fingerprint     TEXT,
  bootstrap_run_id       UUID        REFERENCES runs (id) ON DELETE SET NULL,
  bootstrap_started_at   TIMESTAMPTZ,
  activated_at           TIMESTAMPTZ,
  retired_at             TIMESTAMPTZ,
  error                  TEXT,
  created_by_user_id     UUID        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id     UUID        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id     UUID        REFERENCES users (id) ON DELETE SET NULL,
  created_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at             TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (replication_stream_id, resource_name),
  FOREIGN KEY (replication_stream_id, tenant_id)
    REFERENCES replication_streams (id, tenant_id)
    ON DELETE CASCADE
);

CREATE INDEX replication_stream_resources_status_idx
  ON replication_stream_resources (replication_stream_id, status);

-- Non-stream checkpoints keep their existing version-scoped identity. Stream
-- checkpoints opt into continuity through the resource membership row and are
-- unique per stream/resource across compatible pipeline versions.
ALTER TABLE pipeline_resource_checkpoints
  ADD COLUMN replication_stream_resource_id UUID;

ALTER TABLE pipeline_resource_checkpoints
  ADD CONSTRAINT pipeline_resource_checkpoints_stream_resource_fkey
  FOREIGN KEY (replication_stream_resource_id)
  REFERENCES replication_stream_resources (id)
  ON DELETE CASCADE;

CREATE UNIQUE INDEX pipeline_resource_checkpoints_stream_resource_idx
  ON pipeline_resource_checkpoints (replication_stream_resource_id)
  WHERE replication_stream_resource_id IS NOT NULL;

-- Fence one active run per pipeline route across both compatible versions and
-- replacement stream generations. A newly compiled generation waits for an
-- already-running predecessor to finish instead of writing concurrently.
CREATE UNIQUE INDEX runs_active_replication_route_idx
  ON runs (pipeline_id, (request->>'CheckpointRoute'))
  WHERE status IN (0, 1, 5)
    AND nullif(request->>'ReplicationStreamID', '') IS NOT NULL;

-- +goose Down
DROP INDEX runs_active_replication_route_idx;
DROP INDEX pipeline_resource_checkpoints_stream_resource_idx;

ALTER TABLE pipeline_resource_checkpoints
  DROP CONSTRAINT pipeline_resource_checkpoints_stream_resource_fkey,
  DROP COLUMN replication_stream_resource_id;

DROP INDEX replication_stream_resources_status_idx;
DROP TABLE replication_stream_resources;

DROP INDEX replication_streams_sink_connection_idx;
DROP INDEX replication_streams_source_connection_idx;
DROP INDEX replication_streams_active_route_idx;
DROP TABLE replication_streams;
