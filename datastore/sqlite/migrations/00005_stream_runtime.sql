-- +goose Up
-- Mirrors Postgres 00009_stream_runtime without rebuilding existing stream
-- tables, so resource membership, checkpoints, and notifier history survive.
ALTER TABLE pipelines ADD COLUMN execution_mode INTEGER NOT NULL DEFAULT 1 CHECK (execution_mode IN (1, 2));

CREATE UNIQUE INDEX runs_id_tenant_unique ON runs (id, tenant_id);
ALTER TABLE replication_streams
  ADD COLUMN membership_revision INTEGER NOT NULL DEFAULT 1 CHECK (membership_revision > 0);
ALTER TABLE replication_streams
  ADD COLUMN current_run_id TEXT REFERENCES runs (id) ON DELETE RESTRICT;
ALTER TABLE replication_streams
  ADD COLUMN desired_state TEXT NOT NULL DEFAULT 'stopped' CHECK (desired_state IN ('enabled', 'paused', 'stopped'));
ALTER TABLE replication_streams
  ADD COLUMN desired_revision INTEGER NOT NULL DEFAULT 0 CHECK (desired_revision >= 0);
ALTER TABLE replication_streams ADD COLUMN run_spec TEXT;
ALTER TABLE replication_streams
  ADD COLUMN last_epoch INTEGER NOT NULL DEFAULT 0 CHECK (last_epoch >= 0)
  CONSTRAINT replication_streams_execution_check CHECK (
    (current_run_id IS NULL AND run_spec IS NULL AND desired_revision = 0 AND desired_state = 'stopped' AND last_epoch = 0)
    OR (current_run_id IS NOT NULL AND run_spec IS NOT NULL AND desired_revision > 0)
  );

-- SQLite cannot add a composite foreign key to an existing table. The ID
-- foreign key above retains runs; these triggers enforce the tenant pairing
-- on both sides, equivalent to Postgres's (current_run_id, tenant_id) FK.
-- +goose StatementBegin
CREATE TRIGGER replication_streams_current_run_tenant_insert
BEFORE INSERT ON replication_streams
WHEN NEW.current_run_id IS NOT NULL AND NOT EXISTS (
  SELECT 1 FROM runs WHERE id = NEW.current_run_id AND tenant_id = NEW.tenant_id
)
BEGIN
  SELECT RAISE(ABORT, 'replication stream current run must belong to its tenant');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER replication_streams_current_run_tenant_update
BEFORE UPDATE OF current_run_id, tenant_id ON replication_streams
WHEN NEW.current_run_id IS NOT NULL AND NOT EXISTS (
  SELECT 1 FROM runs WHERE id = NEW.current_run_id AND tenant_id = NEW.tenant_id
)
BEGIN
  SELECT RAISE(ABORT, 'replication stream current run must belong to its tenant');
END;
-- +goose StatementEnd

-- +goose StatementBegin
CREATE TRIGGER runs_stream_tenant_update
BEFORE UPDATE OF tenant_id ON runs
WHEN NEW.tenant_id <> OLD.tenant_id AND EXISTS (
  SELECT 1 FROM replication_streams WHERE current_run_id = OLD.id
)
BEGIN
  SELECT RAISE(ABORT, 'cannot change tenant of a stream current run');
END;
-- +goose StatementEnd

CREATE TABLE stream_attempts (
 -- Fencing tokens increase across every stream and generation.
 token INTEGER PRIMARY KEY AUTOINCREMENT,
 stream_id TEXT NOT NULL,
 tenant_id TEXT NOT NULL,
 execution_id TEXT NOT NULL CHECK (execution_id <> ''),
 desired_revision INTEGER NOT NULL CHECK (desired_revision > 0),
 request_ttl_us INTEGER NOT NULL CHECK (request_ttl_us > 0),
 started_at INTEGER NOT NULL DEFAULT (CAST(unixepoch('subsec') * 1000 AS INTEGER)),
 claimed_at INTEGER,
 run_spec TEXT NOT NULL,
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
DROP TABLE stream_epochs;
DROP INDEX replication_streams_reconcile_idx;
DROP TABLE stream_attempts;
DROP TRIGGER runs_stream_tenant_update;
DROP TRIGGER replication_streams_current_run_tenant_update;
DROP TRIGGER replication_streams_current_run_tenant_insert;
ALTER TABLE replication_streams DROP COLUMN last_epoch;
ALTER TABLE replication_streams DROP COLUMN run_spec;
ALTER TABLE replication_streams DROP COLUMN desired_revision;
ALTER TABLE replication_streams DROP COLUMN desired_state;
ALTER TABLE replication_streams DROP COLUMN current_run_id;
ALTER TABLE replication_streams DROP COLUMN membership_revision;
DROP INDEX runs_id_tenant_unique;
ALTER TABLE pipelines DROP COLUMN execution_mode;
