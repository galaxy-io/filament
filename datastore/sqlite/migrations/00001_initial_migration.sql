-- +goose Up
-- Baseline translated from datastore/postgres migrations 00001-00005.
-- Times are unix milliseconds (INTEGER, NULL = unset), ids are TEXT UUIDs
-- minted in Go, and JSON columns are TEXT.

CREATE TABLE tenants (
  id                 TEXT    PRIMARY KEY,
  external_id        TEXT    UNIQUE,
  name               TEXT,
  created_by_user_id TEXT,
  updated_by_user_id TEXT,
  deleted_by_user_id TEXT,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
);

CREATE TABLE users (
  id                 TEXT    PRIMARY KEY,
  tenant_id          TEXT    NOT NULL REFERENCES tenants (id),
  external_id        TEXT    NOT NULL,
  created_by_user_id TEXT,
  updated_by_user_id TEXT,
  deleted_by_user_id TEXT,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL,
  UNIQUE (tenant_id, external_id)
);

CREATE TABLE connections (
  id                 TEXT    PRIMARY KEY,
  tenant_id          TEXT    NOT NULL REFERENCES tenants (id),
  kind               TEXT    NOT NULL CHECK (kind IN ('source', 'sink')),
  name               TEXT    NOT NULL,
  connector          TEXT    NOT NULL,
  config             TEXT    NOT NULL DEFAULT '{}',
  secret_refs        TEXT    NOT NULL DEFAULT '{}',
  version            INTEGER NOT NULL DEFAULT 1,
  is_deleted         INTEGER NOT NULL DEFAULT 0,
  deleted_at         INTEGER,
  created_by_user_id TEXT,
  updated_by_user_id TEXT,
  deleted_by_user_id TEXT,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL
);

CREATE INDEX connections_tenant_kind_idx ON connections (tenant_id, kind);
CREATE UNIQUE INDEX connections_tenant_name_kind_idx
  ON connections (tenant_id, kind, name) WHERE is_deleted = 0;
CREATE INDEX connections_list_name_idx
  ON connections (tenant_id, kind, lower(name), id) WHERE is_deleted = 0;

CREATE TABLE pipelines (
  id                   TEXT    PRIMARY KEY,
  tenant_id            TEXT    NOT NULL REFERENCES tenants (id),
  name                 TEXT    NOT NULL,
  description          TEXT    NOT NULL DEFAULT '',
  current_version_id   TEXT,
  worker_configuration TEXT    NOT NULL DEFAULT '{}',
  is_deleted           INTEGER NOT NULL DEFAULT 0,
  deleted_at           INTEGER,
  created_by_user_id   TEXT,
  updated_by_user_id   TEXT,
  deleted_by_user_id   TEXT,
  created_at           INTEGER NOT NULL,
  updated_at           INTEGER NOT NULL
);

CREATE INDEX pipelines_tenant_idx ON pipelines (tenant_id);
CREATE INDEX pipelines_list_name_idx
  ON pipelines (tenant_id, lower(name), id) WHERE is_deleted = 0;

CREATE TABLE pipeline_versions (
  id                 TEXT    PRIMARY KEY,
  tenant_id          TEXT    NOT NULL REFERENCES tenants (id),
  pipeline_id        TEXT    NOT NULL REFERENCES pipelines (id) ON DELETE CASCADE,
  version            INTEGER NOT NULL,
  graph              TEXT    NOT NULL,
  created_by_user_id TEXT,
  updated_by_user_id TEXT,
  deleted_by_user_id TEXT,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL,
  UNIQUE (pipeline_id, version)
);

CREATE INDEX pipeline_versions_created_idx
  ON pipeline_versions (pipeline_id, created_at, id);

CREATE TABLE schedules (
  id                 TEXT    PRIMARY KEY,
  tenant_id          TEXT    NOT NULL REFERENCES tenants (id),
  pipeline_id        TEXT    NOT NULL REFERENCES pipelines (id) ON DELETE CASCADE,
  name               TEXT,
  cron_expr          TEXT    NOT NULL,
  timezone           TEXT    NOT NULL DEFAULT 'UTC',
  overlap_policy     INTEGER NOT NULL,
  enabled            INTEGER NOT NULL DEFAULT 1,
  last_fired_at      INTEGER,
  next_fire_at       INTEGER,
  claimed_at         INTEGER,
  created_by_user_id TEXT,
  updated_by_user_id TEXT,
  deleted_by_user_id TEXT,
  created_at         INTEGER NOT NULL,
  updated_at         INTEGER NOT NULL,
  UNIQUE (pipeline_id)
);

CREATE INDEX schedules_due_idx ON schedules (next_fire_at) WHERE enabled = 1;
CREATE INDEX schedules_tenant_idx ON schedules (tenant_id);

CREATE TABLE runs (
  id                  TEXT    PRIMARY KEY,
  tenant_id           TEXT    NOT NULL REFERENCES tenants (id),
  pipeline_id         TEXT,
  pipeline_version_id TEXT,
  schedule_id         TEXT    REFERENCES schedules (id) ON DELETE SET NULL,
  status              INTEGER NOT NULL,
  request             TEXT    NOT NULL,
  records             INTEGER NOT NULL DEFAULT 0,
  bytes               INTEGER NOT NULL DEFAULT 0,
  cpu_seconds         REAL    NOT NULL DEFAULT 0,
  memory_peak_bytes   INTEGER NOT NULL DEFAULT 0,
  scheduled_at        INTEGER,
  requested_at        INTEGER,
  started_at          INTEGER,
  ended_at            INTEGER,
  error               TEXT,
  created_by_user_id  TEXT,
  updated_by_user_id  TEXT,
  deleted_by_user_id  TEXT,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL,
  -- A version requires its pipeline; a scheduled run is pre-created with a
  -- pipeline before any version is compiled. (Postgres's stricter two-way
  -- check predates scheduled pre-creation and would reject the same row.)
  CHECK (pipeline_id IS NOT NULL OR pipeline_version_id IS NULL)
);

CREATE INDEX runs_tenant_created_idx ON runs (tenant_id, created_at DESC);
CREATE INDEX runs_schedule_idx ON runs (schedule_id) WHERE schedule_id IS NOT NULL;
CREATE INDEX runs_metrics_idx ON runs (tenant_id, pipeline_id, pipeline_version_id, status, started_at);
CREATE UNIQUE INDEX runs_idempotency_idx
  ON runs (tenant_id, json_extract(request, '$.IdempotencyKey'))
  WHERE json_extract(request, '$.IdempotencyKey') IS NOT NULL
    AND json_extract(request, '$.IdempotencyKey') != '';
CREATE UNIQUE INDEX runs_active_checkpoint_route_idx
  ON runs (pipeline_id, pipeline_version_id, json_extract(request, '$.CheckpointRoute'))
  WHERE status IN (0, 1, 5)
    AND json_extract(request, '$.CheckpointRoute') IS NOT NULL
    AND json_extract(request, '$.CheckpointRoute') != '';
CREATE INDEX runs_tenant_effective_time_idx
  ON runs (tenant_id, coalesce(started_at, requested_at, scheduled_at, created_at) DESC, id DESC);
CREATE INDEX runs_pipeline_effective_time_idx
  ON runs (pipeline_id, coalesce(started_at, requested_at, scheduled_at, created_at) DESC, id DESC);

CREATE TABLE run_resource_states (
  run_id        TEXT    NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
  resource_name TEXT    NOT NULL,
  tenant_id     TEXT    NOT NULL REFERENCES tenants (id),
  status        INTEGER NOT NULL,
  records       INTEGER NOT NULL DEFAULT 0,
  bytes         INTEGER NOT NULL DEFAULT 0,
  error         TEXT,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  PRIMARY KEY (run_id, resource_name)
);

CREATE TABLE run_resource_checkpoints (
  tenant_id     TEXT    NOT NULL REFERENCES tenants (id),
  run_id        TEXT    NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
  resource_name TEXT    NOT NULL,
  cursor        TEXT    NOT NULL,
  created_at    INTEGER NOT NULL,
  updated_at    INTEGER NOT NULL,
  PRIMARY KEY (run_id, resource_name)
);

CREATE TABLE pipeline_resource_checkpoints (
  tenant_id           TEXT    NOT NULL REFERENCES tenants (id),
  pipeline_id         TEXT    NOT NULL,
  pipeline_version_id TEXT    NOT NULL,
  route_key           TEXT    NOT NULL,
  resource_name       TEXT    NOT NULL,
  cursor              TEXT    NOT NULL,
  last_run_id         TEXT    NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
  created_at          INTEGER NOT NULL,
  updated_at          INTEGER NOT NULL,
  PRIMARY KEY (pipeline_id, pipeline_version_id, route_key, resource_name)
);

CREATE INDEX pipeline_resource_checkpoints_last_run_idx
  ON pipeline_resource_checkpoints (last_run_id);

CREATE TABLE run_dedup_seen (
  tenant_id  TEXT    NOT NULL REFERENCES tenants (id),
  run_id     TEXT    NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
  last_seq   INTEGER NOT NULL,
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  PRIMARY KEY (tenant_id, run_id)
);

CREATE TABLE secrets (
  tenant_id  TEXT    NOT NULL REFERENCES tenants (id),
  ref        TEXT    NOT NULL PRIMARY KEY,
  ciphertext BLOB    NOT NULL,
  nonce      BLOB    NOT NULL,
  key_id     TEXT    NOT NULL,
  metadata   TEXT    NOT NULL DEFAULT '{}',
  created_at INTEGER NOT NULL,
  updated_at INTEGER NOT NULL,
  UNIQUE (tenant_id, ref)
);

-- The built-in tenant used when callers omit one (filament.DefaultTenantID);
-- local deployments run single-tenant against it.
INSERT INTO tenants (id, name, created_at, updated_at)
VALUES ('00000000-0000-0000-0000-000000000000', 'default', 0, 0);

-- +goose Down
DROP TABLE secrets;
DROP TABLE run_dedup_seen;
DROP TABLE pipeline_resource_checkpoints;
DROP TABLE run_resource_checkpoints;
DROP TABLE run_resource_states;
DROP TABLE runs;
DROP TABLE schedules;
DROP TABLE pipeline_versions;
DROP TABLE pipelines;
DROP TABLE connections;
DROP TABLE users;
DROP TABLE tenants;
