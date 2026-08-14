-- +goose Up
CREATE TABLE tenants (
  id                 TEXT        PRIMARY KEY,
  name               TEXT,
  created_by_user_id TEXT,
  updated_by_user_id TEXT,
  deleted_by_user_id TEXT,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE users (
  id                 TEXT        PRIMARY KEY,
  tenant_id          TEXT        NOT NULL REFERENCES tenants (id),
  external_id        TEXT        NOT NULL,
  created_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, external_id)
);

ALTER TABLE tenants
  ADD CONSTRAINT tenants_created_by_user_fkey FOREIGN KEY (created_by_user_id) REFERENCES users (id) ON DELETE SET NULL,
  ADD CONSTRAINT tenants_updated_by_user_fkey FOREIGN KEY (updated_by_user_id) REFERENCES users (id) ON DELETE SET NULL,
  ADD CONSTRAINT tenants_deleted_by_user_fkey FOREIGN KEY (deleted_by_user_id) REFERENCES users (id) ON DELETE SET NULL;

CREATE TYPE connection_kind AS ENUM ('source', 'sink');

CREATE TABLE connections (
  id                 TEXT            PRIMARY KEY,
  tenant_id          TEXT            NOT NULL REFERENCES tenants (id),
  kind               connection_kind NOT NULL,
  name               TEXT            NOT NULL,
  provider           TEXT            NOT NULL,
  config             JSONB           NOT NULL DEFAULT '{}',
  secret_refs        JSONB           NOT NULL DEFAULT '{}',
  version            BIGINT          NOT NULL DEFAULT 1,
  is_deleted         BOOLEAN         NOT NULL DEFAULT false,
  deleted_at         TIMESTAMPTZ,
  created_by_user_id TEXT            REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT            REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT            REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ     NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ     NOT NULL DEFAULT now()
);

CREATE INDEX connections_tenant_kind_idx ON connections (tenant_id, kind);
CREATE UNIQUE INDEX connections_tenant_name_kind_idx
  ON connections (tenant_id, kind, name) WHERE NOT is_deleted;

CREATE TABLE pipelines (
  id                 TEXT        PRIMARY KEY,
  tenant_id          TEXT        NOT NULL REFERENCES tenants (id),
  name               TEXT        NOT NULL,
  description        TEXT        NOT NULL DEFAULT '',
  current_version_id TEXT,
  worker_configuration JSONB     NOT NULL DEFAULT '{}',
  is_deleted         BOOLEAN     NOT NULL DEFAULT false,
  deleted_at         TIMESTAMPTZ,
  created_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (id, tenant_id)
);

CREATE INDEX pipelines_tenant_idx ON pipelines (tenant_id);

CREATE TABLE pipeline_versions (
  id                 TEXT        PRIMARY KEY,
  pipeline_id        TEXT        NOT NULL REFERENCES pipelines (id) ON DELETE CASCADE,
  version            BIGINT      NOT NULL,
  graph              JSONB       NOT NULL,
  created_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (pipeline_id, version),
  UNIQUE (pipeline_id, id)
);

ALTER TABLE pipelines
  ADD CONSTRAINT pipelines_current_version_fkey
  FOREIGN KEY (id, current_version_id)
  REFERENCES pipeline_versions (pipeline_id, id);

CREATE TABLE schedules (
  id                 TEXT        PRIMARY KEY,
  tenant_id          TEXT        NOT NULL REFERENCES tenants (id),
  pipeline_id        TEXT        NOT NULL,
  name               TEXT,
  cron_expr          TEXT        NOT NULL,
  timezone           TEXT        NOT NULL DEFAULT 'UTC',
  overlap_policy     SMALLINT    NOT NULL,
  enabled            BOOLEAN     NOT NULL DEFAULT true,
  last_fired_at      TIMESTAMPTZ,
  next_fire_at       TIMESTAMPTZ,
  claimed_at         TIMESTAMPTZ,
  created_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (pipeline_id, tenant_id) REFERENCES pipelines (id, tenant_id) ON DELETE CASCADE,
  UNIQUE (pipeline_id)
);

CREATE INDEX schedules_due_idx ON schedules (next_fire_at) WHERE enabled = true;
CREATE INDEX schedules_tenant_idx ON schedules (tenant_id);

CREATE TABLE runs (
  id                 TEXT        PRIMARY KEY,
  tenant_id          TEXT        NOT NULL REFERENCES tenants (id),
  pipeline_id        TEXT,
  pipeline_version_id TEXT,
  schedule_id        TEXT        REFERENCES schedules (id) ON DELETE SET NULL,
  status             SMALLINT    NOT NULL,
  request            JSONB       NOT NULL,
  records            BIGINT      NOT NULL DEFAULT 0,
  bytes              BIGINT      NOT NULL DEFAULT 0,
  cpu_seconds        DOUBLE PRECISION NOT NULL DEFAULT 0,
  memory_peak_bytes  BIGINT      NOT NULL DEFAULT 0,
  scheduled_at       TIMESTAMPTZ,
  requested_at       TIMESTAMPTZ,
  started_at         TIMESTAMPTZ,
  ended_at           TIMESTAMPTZ,
  error              TEXT,
  created_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (pipeline_id, pipeline_version_id)
    REFERENCES pipeline_versions (pipeline_id, id),
  CHECK ((pipeline_id IS NULL) = (pipeline_version_id IS NULL))
);

CREATE INDEX runs_tenant_created_idx ON runs (tenant_id, created_at DESC);
CREATE INDEX runs_schedule_idx ON runs (schedule_id) WHERE schedule_id IS NOT NULL;
CREATE INDEX runs_metrics_idx ON runs (tenant_id, pipeline_id, pipeline_version_id, status, started_at);
CREATE UNIQUE INDEX runs_idempotency_idx ON runs (tenant_id, (request->>'IdempotencyKey'))
  WHERE nullif(request->>'IdempotencyKey', '') IS NOT NULL;
CREATE UNIQUE INDEX runs_active_checkpoint_route_idx
  ON runs (pipeline_id, pipeline_version_id, (request->>'CheckpointRoute'))
  WHERE status IN (0, 1, 5)
    AND nullif(request->>'CheckpointRoute', '') IS NOT NULL;

CREATE TABLE run_resource_states (
  id                 TEXT        PRIMARY KEY,
  run_id             TEXT        NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
  resource_name      TEXT        NOT NULL,
  tenant_id          TEXT        NOT NULL REFERENCES tenants (id),
  status             SMALLINT    NOT NULL,
  records            BIGINT      NOT NULL DEFAULT 0,
  bytes              BIGINT      NOT NULL DEFAULT 0,
  error              TEXT,
  created_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (run_id, resource_name)
);

CREATE TABLE run_resource_checkpoints (
  id                 TEXT        PRIMARY KEY,
  run_id             TEXT        NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
  resource_name      TEXT        NOT NULL,
  cursor             JSONB       NOT NULL,
  created_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (run_id, resource_name)
);

CREATE TABLE pipeline_resource_checkpoints (
  id                  TEXT        PRIMARY KEY,
  pipeline_id         TEXT        NOT NULL,
  pipeline_version_id TEXT        NOT NULL,
  route_key           TEXT        NOT NULL,
  resource_name       TEXT        NOT NULL,
  cursor              JSONB       NOT NULL,
  last_run_id         TEXT        NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
  created_by_user_id  TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id  TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id  TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
  FOREIGN KEY (pipeline_id, pipeline_version_id)
    REFERENCES pipeline_versions (pipeline_id, id) ON DELETE CASCADE,
  UNIQUE (pipeline_id, pipeline_version_id, route_key, resource_name)
);

CREATE INDEX pipeline_resource_checkpoints_last_run_idx
  ON pipeline_resource_checkpoints (last_run_id);

CREATE TABLE run_dedup_seen (
  id                 TEXT        PRIMARY KEY,
  tenant_id          TEXT        NOT NULL REFERENCES tenants (id),
  run_id             TEXT        NOT NULL REFERENCES runs (id) ON DELETE CASCADE,
  last_seq           BIGINT      NOT NULL,
  created_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, run_id)
);

CREATE TABLE secrets (
  id                 TEXT        PRIMARY KEY,
  tenant_id          TEXT        NOT NULL REFERENCES tenants (id),
  ref                TEXT        NOT NULL,
  ciphertext         BYTEA       NOT NULL,
  nonce              BYTEA       NOT NULL,
  key_id             TEXT        NOT NULL,
  metadata           JSONB       NOT NULL DEFAULT '{}',
  created_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  updated_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  deleted_by_user_id TEXT        REFERENCES users (id) ON DELETE SET NULL,
  created_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at         TIMESTAMPTZ NOT NULL DEFAULT now(),
  UNIQUE (tenant_id, ref),
  UNIQUE (ref)
);

-- +goose Down
DROP TABLE secrets;
DROP TABLE run_dedup_seen;
DROP TABLE pipeline_resource_checkpoints;
DROP TABLE run_resource_checkpoints;
DROP TABLE run_resource_states;
DROP TABLE runs;
DROP TABLE schedules;
ALTER TABLE pipelines DROP CONSTRAINT pipelines_current_version_fkey;
DROP TABLE pipeline_versions;
DROP TABLE pipelines;
DROP TABLE connections;
DROP TYPE connection_kind;
ALTER TABLE tenants
  DROP CONSTRAINT tenants_created_by_user_fkey,
  DROP CONSTRAINT tenants_updated_by_user_fkey,
  DROP CONSTRAINT tenants_deleted_by_user_fkey;
DROP TABLE users;
DROP TABLE tenants;
