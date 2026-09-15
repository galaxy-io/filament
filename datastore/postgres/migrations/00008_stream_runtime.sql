-- +goose Up
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
 desired_revision BIGINT NOT NULL,
 request_ttl_us BIGINT NOT NULL CHECK (request_ttl_us > 0),
 started_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 expires_at TIMESTAMPTZ NOT NULL,
 ended_at TIMESTAMPTZ,
 termination TEXT CHECK (termination IN ('clean','reaped','unproven')),
 reason TEXT NOT NULL DEFAULT '',
 CHECK ((ended_at IS NULL) = (termination IS NULL)),
 FOREIGN KEY (stream_id, tenant_id) REFERENCES replication_streams(id, tenant_id),
 UNIQUE (tenant_id, execution_id)
);
CREATE UNIQUE INDEX stream_attempts_live_idx ON stream_attempts(stream_id) WHERE ended_at IS NULL;
CREATE INDEX stream_attempts_stream_idx ON stream_attempts(stream_id, token DESC);
CREATE INDEX replication_streams_reconcile_idx ON replication_streams(tenant_id, (id::text)) WHERE status = 0 AND current_run_id IS NOT NULL;

-- Serialize membership changes with certification on the parent stream row.
-- Multi-row membership writers must lock the parent stream before child rows.
-- Runtime writers additionally lock the pipeline first for cross-generation admission.
-- The trigger also covers legacy resource checkpoint writers; timestamp-only updates
-- do not change membership. Keep this ordering when adding mutation paths.
-- +goose StatementBegin
CREATE FUNCTION bump_stream_membership_revision() RETURNS TRIGGER LANGUAGE plpgsql AS $$
BEGIN
 IF TG_OP = 'UPDATE' AND ROW(NEW.replication_stream_id, NEW.tenant_id, NEW.resource_name,
  NEW.status, NEW.bootstrap_mode, NEW.bootstrap_config, NEW.schema_fingerprint)
  IS NOT DISTINCT FROM ROW(OLD.replication_stream_id, OLD.tenant_id, OLD.resource_name,
  OLD.status, OLD.bootstrap_mode, OLD.bootstrap_config, OLD.schema_fingerprint) THEN RETURN NEW; END IF;
 IF TG_OP = 'UPDATE' AND NEW.replication_stream_id <> OLD.replication_stream_id THEN
  RAISE EXCEPTION 'resource membership cannot move between streams';
 END IF;
 UPDATE replication_streams SET membership_revision = membership_revision + 1
 WHERE id = CASE WHEN TG_OP = 'DELETE' THEN OLD.replication_stream_id ELSE NEW.replication_stream_id END;
 IF TG_OP = 'DELETE' THEN RETURN OLD; END IF;
 RETURN NEW;
END $$;
-- +goose StatementEnd
CREATE TRIGGER stream_membership_revision_trigger AFTER INSERT OR UPDATE OR DELETE ON replication_stream_resources
 FOR EACH ROW EXECUTE FUNCTION bump_stream_membership_revision();

CREATE TABLE stream_epochs (
 stream_id UUID NOT NULL,
 tenant_id UUID NOT NULL,
 epoch BIGINT NOT NULL CHECK (epoch > 0),
 attempt_token BIGINT NOT NULL REFERENCES stream_attempts(token),
 certificate BYTEA NOT NULL,
 -- Cumulative committed position per domain after this epoch: the previous
 -- epoch's snapshot merged with this certificate's coverage.
 positions JSONB NOT NULL,
 committed_at TIMESTAMPTZ NOT NULL DEFAULT clock_timestamp(),
 PRIMARY KEY (stream_id, epoch),
 FOREIGN KEY (stream_id, tenant_id) REFERENCES replication_streams(id, tenant_id)
);

-- +goose Down
-- Application rollback retains progress, attempt history and issued fencing tokens.
-- +goose StatementBegin
DO $$ BEGIN
 RAISE EXCEPTION 'stream runtime cannot be downgraded; retain durable state on application rollback';
END $$;
-- +goose StatementEnd
