-- +goose Up
CREATE TABLE runs (
  run_id          TEXT        PRIMARY KEY,
  tenant_id       TEXT        NOT NULL,
  schedule_id     TEXT,
  status          SMALLINT    NOT NULL,
  request         JSONB       NOT NULL,
  records         BIGINT      NOT NULL DEFAULT 0,
  bytes           BIGINT      NOT NULL DEFAULT 0,
  started_at      TIMESTAMPTZ,
  finished_at     TIMESTAMPTZ,
  error           TEXT,
  created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
  source_provider TEXT GENERATED ALWAYS AS (request->'Source'->>'Provider') STORED,
  ingestion_type  TEXT GENERATED ALWAYS AS (request->>'IngestionType') STORED
);

CREATE INDEX runs_tenant_created_idx ON runs (tenant_id, created_at DESC);
CREATE INDEX runs_schedule_idx ON runs (schedule_id) WHERE schedule_id IS NOT NULL;
CREATE UNIQUE INDEX runs_idempotency_idx ON runs (tenant_id, (request->>'IdempotencyKey'))
  WHERE nullif(request->>'IdempotencyKey', '') IS NOT NULL;

-- +goose Down
DROP TABLE runs;
