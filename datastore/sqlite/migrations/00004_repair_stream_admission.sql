-- +goose Up
-- Mirrors Postgres 00008_repair_stream_admission.
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

DROP INDEX runs_active_replication_route_idx;

CREATE UNIQUE INDEX runs_active_replication_route_idx
  ON runs (pipeline_id, json_extract(request, '$.CheckpointRoute'))
  WHERE status IN (0, 1, 5)
    AND coalesce(nullif(json_extract(request, '$.ReplicationStream.ID'), ''), nullif(json_extract(request, '$.ReplicationStreamID'), '')) IS NOT NULL;

-- +goose Down
-- Retain the admission fence on application rollback, matching Postgres 00008.
CREATE TABLE stream_admission_downgrade_guard (
  valid                            INTEGER CONSTRAINT stream_admission_cannot_be_downgraded CHECK (valid = 1)
);
INSERT INTO stream_admission_downgrade_guard (valid) VALUES (0);
