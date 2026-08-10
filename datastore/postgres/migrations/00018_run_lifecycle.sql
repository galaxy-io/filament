-- +goose Up
-- Splits the run lifecycle into the distinct moments started_at was standing in
-- for. runs.Submit stamped started_at at submission and the tracker's run.started
-- fold was guarded on IsZero, so it never fired: started_at has always held
-- request time, and every duration derived from it silently included queue wait
-- and worker spin-up.
--
-- created_at and updated_at already exist (00002) and are simply read now.
--
-- Not GENERATED like source_provider/pipeline_id (00002, 00012): text ->
-- timestamptz is STABLE rather than IMMUTABLE (it reads the TimeZone setting),
-- so Postgres rejects it in a generated column. Both are written explicitly by
-- the module that owns them.
ALTER TABLE runs ADD COLUMN scheduled_at TIMESTAMPTZ;
ALTER TABLE runs ADD COLUMN requested_at TIMESTAMPTZ;

-- started_at has always meant request time, so that is what it backfills.
-- Historical started_at is left as-is: nulling it would discard the only timing
-- those rows carry. The cost is a one-time step change in run duration metrics
-- at deploy, as new runs begin measuring execution rather than end to end.
UPDATE runs SET requested_at = started_at WHERE started_at IS NOT NULL;

-- +goose Down
ALTER TABLE runs DROP COLUMN requested_at;
ALTER TABLE runs DROP COLUMN scheduled_at;
