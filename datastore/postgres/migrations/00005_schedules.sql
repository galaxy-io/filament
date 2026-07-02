-- +goose Up
CREATE TABLE schedules (
  schedule_id     TEXT     PRIMARY KEY,
  tenant_id       TEXT     NOT NULL,
  name            TEXT,
  cron_expr       TEXT     NOT NULL,
  timezone        TEXT     NOT NULL DEFAULT 'UTC',
  jitter_ms       BIGINT   NOT NULL DEFAULT 0,
  overlap_policy  SMALLINT NOT NULL,
  catchup_policy  SMALLINT NOT NULL,
  request         JSONB    NOT NULL,
  enabled         BOOLEAN  NOT NULL DEFAULT true,
  last_fired_at   TIMESTAMPTZ,
  next_fire_at    TIMESTAMPTZ,
  claimed_at      TIMESTAMPTZ,
  last_run_id     TEXT,
  last_run_status SMALLINT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX schedules_due_idx ON schedules (next_fire_at) WHERE enabled = true;
CREATE INDEX schedules_tenant_idx ON schedules (tenant_id);

-- +goose Down
DROP TABLE schedules;
