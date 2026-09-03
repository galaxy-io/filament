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
  consumer_name                    TEXT        NOT NULL CHECK (consumer_name <> ''),
  consumer_config                   JSONB       NOT NULL DEFAULT '{}',
  continuity_fingerprint            TEXT        NOT NULL CHECK (continuity_fingerprint <> ''),
  status                           TEXT        NOT NULL DEFAULT 'active'
                                                CHECK (status IN ('active', 'retired', 'error')),
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
  WHERE status = 'active';

CREATE INDEX replication_streams_source_connection_idx
  ON replication_streams (source_connection_id);

CREATE INDEX replication_streams_sink_connection_idx
  ON replication_streams (sink_connection_id);

-- Resource membership is distinct from checkpoint presence: a newly selected
-- table/topic exists as pending before it owns a cursor, and a retired resource
-- keeps its history without participating in the stream checkpoint floor.
CREATE TABLE replication_stream_resources (
  replication_stream_id UUID        NOT NULL,
  tenant_id              UUID        NOT NULL REFERENCES tenants (id),
  resource_name          TEXT        NOT NULL,
  status                 TEXT        NOT NULL DEFAULT 'pending'
                                      CHECK (status IN ('pending', 'bootstrapping', 'active', 'retired', 'error')),
  bootstrap_mode         TEXT        NOT NULL,
  bootstrap_config        JSONB       NOT NULL DEFAULT '{}',
  schema_fingerprint      TEXT,
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
  PRIMARY KEY (replication_stream_id, resource_name),
  FOREIGN KEY (replication_stream_id, tenant_id)
    REFERENCES replication_streams (id, tenant_id)
    ON DELETE CASCADE
);

CREATE INDEX replication_stream_resources_status_idx
  ON replication_stream_resources (replication_stream_id, status);

-- Non-stream checkpoints keep their existing version-scoped identity. Stream
-- checkpoints opt into continuity through replication_stream_id and are unique
-- per stream/resource across compatible pipeline versions.
ALTER TABLE pipeline_resource_checkpoints
  ADD COLUMN replication_stream_id UUID;

ALTER TABLE pipeline_resource_checkpoints
  ADD CONSTRAINT pipeline_resource_checkpoints_stream_resource_fkey
  FOREIGN KEY (replication_stream_id, resource_name)
  REFERENCES replication_stream_resources (replication_stream_id, resource_name)
  ON DELETE CASCADE;

CREATE UNIQUE INDEX pipeline_resource_checkpoints_stream_resource_idx
  ON pipeline_resource_checkpoints (replication_stream_id, resource_name)
  WHERE replication_stream_id IS NOT NULL;

-- +goose Down
DROP INDEX pipeline_resource_checkpoints_stream_resource_idx;

ALTER TABLE pipeline_resource_checkpoints
  DROP CONSTRAINT pipeline_resource_checkpoints_stream_resource_fkey,
  DROP COLUMN replication_stream_id;

DROP INDEX replication_stream_resources_status_idx;
DROP TABLE replication_stream_resources;

DROP INDEX replication_streams_sink_connection_idx;
DROP INDEX replication_streams_source_connection_idx;
DROP INDEX replication_streams_active_route_idx;
DROP TABLE replication_streams;
