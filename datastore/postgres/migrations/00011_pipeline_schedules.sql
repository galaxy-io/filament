-- +goose Up
ALTER TABLE schedules
  ADD COLUMN pipeline_id TEXT;

UPDATE runs SET schedule_id = NULL WHERE schedule_id IS NOT NULL;
DELETE FROM schedules;

ALTER TABLE schedules
  ALTER COLUMN pipeline_id SET NOT NULL,
  DROP COLUMN request,
  DROP COLUMN jitter_ms,
  DROP COLUMN catchup_policy,
  DROP COLUMN last_run_id,
  DROP COLUMN last_run_status;

ALTER TABLE pipelines
  ADD CONSTRAINT pipelines_id_tenant_unique UNIQUE (pipeline_id, tenant_id);

ALTER TABLE schedules
  ADD CONSTRAINT schedules_pipeline_tenant_fkey
  FOREIGN KEY (pipeline_id, tenant_id)
  REFERENCES pipelines (pipeline_id, tenant_id)
  ON DELETE CASCADE;

CREATE UNIQUE INDEX schedules_pipeline_unique ON schedules (pipeline_id);

-- +goose Down
DROP INDEX schedules_pipeline_unique;
ALTER TABLE schedules DROP CONSTRAINT schedules_pipeline_tenant_fkey;
ALTER TABLE pipelines DROP CONSTRAINT pipelines_id_tenant_unique;
ALTER TABLE schedules
  ADD COLUMN request JSONB NOT NULL DEFAULT '{}'::jsonb,
  ADD COLUMN jitter_ms BIGINT NOT NULL DEFAULT 0,
  ADD COLUMN catchup_policy SMALLINT NOT NULL DEFAULT 0,
  ADD COLUMN last_run_id TEXT REFERENCES runs (run_id) ON DELETE SET NULL,
  ADD COLUMN last_run_status SMALLINT,
  ALTER COLUMN pipeline_id DROP NOT NULL,
  DROP COLUMN pipeline_id;
