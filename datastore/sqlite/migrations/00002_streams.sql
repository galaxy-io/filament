-- +goose Up
-- Stream schema and runtime from Postgres migrations 00005, 00007, and 00008.
-- IDs are minted in Go; timestamps are unix milliseconds and JSON is TEXT.
CREATE UNIQUE INDEX pipelines_id_tenant_unique ON pipelines (id, tenant_id);
CREATE UNIQUE INDEX pipeline_versions_pipeline_id_unique ON pipeline_versions (pipeline_id, id);
CREATE UNIQUE INDEX runs_id_tenant_unique ON runs (id, tenant_id);

-- A replication stream is the durable identity of one independently advancing
-- source-to-sink consumer. Pipeline versions may reuse the stream when an edit
-- preserves continuity; incompatible edits create the next generation.
CREATE TABLE replication_streams (
  id                               TEXT    PRIMARY KEY NOT NULL,
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
  created_by_user_id               TEXT    REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id               TEXT    REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id               TEXT    REFERENCES users (id) ON DELETE SET NULL,
  created_at                       INTEGER NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),
  updated_at                       INTEGER NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),
  membership_revision              INTEGER NOT NULL DEFAULT 1 CHECK (membership_revision > 0),
  current_run_id                   TEXT,
  desired_state                    TEXT    NOT NULL DEFAULT 'stopped' CHECK (desired_state IN ('enabled', 'paused', 'stopped')),
  desired_revision                 INTEGER NOT NULL DEFAULT 0 CHECK (desired_revision >= 0),
  run_spec                         TEXT,
  last_epoch                       INTEGER NOT NULL DEFAULT 0 CHECK (last_epoch >= 0),
  FOREIGN KEY (current_run_id, tenant_id) REFERENCES runs (id, tenant_id) ON DELETE RESTRICT,
  CHECK (
    (current_run_id IS NULL AND run_spec IS NULL AND desired_revision = 0 AND desired_state = 'stopped' AND last_epoch = 0)
    OR (current_run_id IS NOT NULL AND run_spec IS NOT NULL AND desired_revision > 0)
  ),
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
  id                               TEXT    PRIMARY KEY NOT NULL,
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
  created_by_user_id               TEXT    REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id               TEXT    REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id               TEXT    REFERENCES users (id) ON DELETE SET NULL,
  created_at                       INTEGER NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),
  updated_at                       INTEGER NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),
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
  ADD COLUMN replication_stream_resource_id TEXT
  REFERENCES replication_stream_resources (id)
  ON DELETE CASCADE;

CREATE UNIQUE INDEX pipeline_resource_checkpoints_stream_resource_idx
  ON pipeline_resource_checkpoints (replication_stream_resource_id)
  WHERE replication_stream_resource_id IS NOT NULL;

-- Validate historical requests before installing the admission fence. Goose
-- runs this migration transactionally, so any conflict rolls back the upgrade.
CREATE TABLE stream_admission_validation (
  valid                            INTEGER CONSTRAINT runs_replication_route_identity_check CHECK (valid = 1)
);
INSERT INTO stream_admission_validation (valid)
  SELECT 0 FROM runs
  WHERE coalesce(nullif(json_extract(request, '$.ReplicationStream.ID'), ''), nullif(json_extract(request, '$.ReplicationStreamID'), '')) IS NOT NULL
  AND (pipeline_id IS NULL OR nullif(json_extract(request, '$.CheckpointRoute'), '') IS NULL);
DROP TABLE stream_admission_validation;

-- SQLite cannot add a table CHECK without rebuilding runs and its dependents.
-- These triggers enforce the same identity rule on inserts and updates.
-- +goose StatementBegin
CREATE TRIGGER runs_replication_route_identity_insert
BEFORE INSERT ON runs
WHEN coalesce(nullif(json_extract(NEW.request, '$.ReplicationStream.ID'), ''), nullif(json_extract(NEW.request, '$.ReplicationStreamID'), '')) IS NOT NULL
  AND (NEW.pipeline_id IS NULL OR nullif(json_extract(NEW.request, '$.CheckpointRoute'), '') IS NULL)
BEGIN
  SELECT RAISE(ABORT, 'replication runs have missing pipeline or CheckpointRoute');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER runs_replication_route_identity_update
BEFORE UPDATE ON runs
WHEN coalesce(nullif(json_extract(NEW.request, '$.ReplicationStream.ID'), ''), nullif(json_extract(NEW.request, '$.ReplicationStreamID'), '')) IS NOT NULL
  AND (NEW.pipeline_id IS NULL OR nullif(json_extract(NEW.request, '$.CheckpointRoute'), '') IS NULL)
BEGIN
  SELECT RAISE(ABORT, 'replication runs have missing pipeline or CheckpointRoute');
END;
-- +goose StatementEnd

CREATE UNIQUE INDEX runs_active_replication_route_idx
  ON runs (pipeline_id, json_extract(request, '$.CheckpointRoute'))
  WHERE status IN (0, 1, 5)
    AND coalesce(nullif(json_extract(request, '$.ReplicationStream.ID'), ''), nullif(json_extract(request, '$.ReplicationStreamID'), '')) IS NOT NULL;

CREATE TABLE stream_attempts (
 -- Fencing tokens increase across every stream and generation.
 token INTEGER PRIMARY KEY AUTOINCREMENT,
 stream_id TEXT NOT NULL,
 tenant_id TEXT NOT NULL,
 execution_id TEXT NOT NULL CHECK (execution_id <> ''),
 desired_revision INTEGER NOT NULL CHECK (desired_revision > 0),
 request_ttl_us INTEGER NOT NULL CHECK (request_ttl_us > 0),
 started_at INTEGER NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),
 expires_at INTEGER NOT NULL,
 ended_at INTEGER,
 termination TEXT CHECK (termination IN ('clean','reaped','unproven')),
 reason TEXT NOT NULL DEFAULT '',
 CHECK ((ended_at IS NULL) = (termination IS NULL)),
 FOREIGN KEY (stream_id, tenant_id) REFERENCES replication_streams(id, tenant_id),
 UNIQUE (tenant_id, execution_id),
 UNIQUE (token, stream_id, tenant_id)
);
CREATE UNIQUE INDEX stream_attempts_live_idx ON stream_attempts(stream_id) WHERE ended_at IS NULL;
CREATE INDEX stream_attempts_stream_idx ON stream_attempts(stream_id, token DESC);
CREATE INDEX replication_streams_reconcile_idx ON replication_streams(tenant_id, id) WHERE status = 0 AND current_run_id IS NOT NULL;

CREATE TABLE stream_epochs (
 stream_id TEXT NOT NULL,
 tenant_id TEXT NOT NULL,
 epoch INTEGER NOT NULL CHECK (epoch > 0),
 attempt_token INTEGER NOT NULL,
 certificate BLOB NOT NULL,
 -- Cumulative committed position per domain after this epoch: the previous
 -- epoch's snapshot merged with this certificate's coverage.
 positions TEXT NOT NULL,
 committed_at INTEGER NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),
 PRIMARY KEY (stream_id, epoch),
 FOREIGN KEY (stream_id, tenant_id) REFERENCES replication_streams(id, tenant_id),
 FOREIGN KEY (attempt_token, stream_id, tenant_id)
  REFERENCES stream_attempts(token, stream_id, tenant_id)
);

-- +goose Down
-- Retain the admission fence on application rollback, matching Postgres 00007.
CREATE TABLE stream_admission_downgrade_guard (
  valid                            INTEGER CONSTRAINT stream_admission_cannot_be_downgraded CHECK (valid = 1)
);
INSERT INTO stream_admission_downgrade_guard (valid) VALUES (0);
