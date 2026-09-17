-- +goose Up
ALTER TABLE pipelines ADD COLUMN execution INTEGER NOT NULL DEFAULT 1 CHECK (execution IN (1, 2));

ALTER TABLE runs ADD CONSTRAINT runs_id_tenant_unique UNIQUE (id, tenant_id);
ALTER TABLE replication_streams ADD COLUMN membership_revision BIGINT NOT NULL DEFAULT 1 CHECK (membership_revision > 0);

-- Existing bounded replication streams have no continuous execution intent.
ALTER TABLE replication_streams
 ADD COLUMN current_run_id UUID,
 ADD COLUMN desired_state TEXT NOT NULL DEFAULT 'stopped' CHECK (desired_state IN ('enabled','paused','stopped')),
 ADD COLUMN desired_revision BIGINT NOT NULL DEFAULT 0 CHECK (desired_revision >= 0),
 ADD COLUMN run_spec JSONB,
 ADD COLUMN last_epoch BIGINT NOT NULL DEFAULT 0 CHECK (last_epoch >= 0),
 -- Referenced runs are retained even after stop; history cleanup is explicit future work.
 ADD CONSTRAINT replication_streams_current_run_fkey
  FOREIGN KEY (current_run_id, tenant_id) REFERENCES runs(id, tenant_id) ON DELETE RESTRICT,
 ADD CONSTRAINT replication_streams_execution_check CHECK (
  (current_run_id IS NULL AND run_spec IS NULL AND desired_revision = 0 AND desired_state = 'stopped' AND last_epoch = 0)
  OR (current_run_id IS NOT NULL AND run_spec IS NOT NULL AND desired_revision > 0)
 );
CREATE TABLE stream_attempts (
 -- Fencing tokens increase across every stream and generation.
 token BIGINT PRIMARY KEY GENERATED ALWAYS AS IDENTITY,
 stream_id UUID NOT NULL,
 tenant_id UUID NOT NULL,
 execution_id TEXT NOT NULL CHECK (execution_id <> ''),
 desired_revision BIGINT NOT NULL CHECK (desired_revision > 0),
 request_ttl_us BIGINT NOT NULL CHECK (request_ttl_us > 0),
 started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 claimed_at TIMESTAMPTZ,
 run_spec JSONB NOT NULL,
 expires_at TIMESTAMPTZ NOT NULL,
 ended_at TIMESTAMPTZ,
 termination TEXT CHECK (termination IN ('clean','reaped','unproven')),
 reason TEXT NOT NULL DEFAULT '',
 CHECK ((ended_at IS NULL) = (termination IS NULL)),
 FOREIGN KEY (stream_id, tenant_id) REFERENCES replication_streams(id, tenant_id),
 UNIQUE (tenant_id, execution_id),
 UNIQUE (token, stream_id, tenant_id)
);
CREATE UNIQUE INDEX stream_attempts_live_idx ON stream_attempts(stream_id) WHERE ended_at IS NULL;
CREATE INDEX stream_attempts_stream_idx ON stream_attempts(stream_id, token DESC);
CREATE INDEX replication_streams_reconcile_idx ON replication_streams(tenant_id, (id::text)) WHERE status = 0 AND current_run_id IS NOT NULL;

CREATE TABLE stream_epochs (
 stream_id UUID NOT NULL,
 tenant_id UUID NOT NULL,
 epoch BIGINT NOT NULL CHECK (epoch > 0),
 attempt_token BIGINT NOT NULL,
 certificate BYTEA NOT NULL,
 -- Cumulative committed position per domain after this epoch: the previous
 -- epoch's snapshot merged with this certificate's coverage.
 positions JSONB NOT NULL,
 committed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (stream_id, epoch),
 FOREIGN KEY (stream_id, tenant_id) REFERENCES replication_streams(id, tenant_id),
 FOREIGN KEY (attempt_token, stream_id, tenant_id)
  REFERENCES stream_attempts(token, stream_id, tenant_id)
);

-- +goose Down
DROP TABLE stream_epochs;
DROP INDEX replication_streams_reconcile_idx;
DROP TABLE stream_attempts;

ALTER TABLE replication_streams
 DROP CONSTRAINT replication_streams_execution_check,
 DROP CONSTRAINT replication_streams_current_run_fkey,
 DROP COLUMN last_epoch,
 DROP COLUMN run_spec,
 DROP COLUMN desired_revision,
 DROP COLUMN desired_state,
 DROP COLUMN current_run_id,
 DROP COLUMN membership_revision;

ALTER TABLE runs DROP CONSTRAINT runs_id_tenant_unique;
ALTER TABLE pipelines DROP COLUMN execution;
