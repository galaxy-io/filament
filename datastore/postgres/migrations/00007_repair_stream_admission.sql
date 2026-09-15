-- +goose Up
-- Hold the admission fence stable through diagnostics and index replacement.
LOCK TABLE runs IN ACCESS EXCLUSIVE MODE;

-- Report conflicts without selecting a winner or stopping workers.
-- +goose StatementBegin
DO $$
DECLARE route_conflicts TEXT;
BEGIN
  SELECT string_agg(format('pipeline=%s route=%s runs=%s', pipeline_id, route, ids), '; ')
    INTO route_conflicts
  FROM (
    SELECT pipeline_id, request->>'CheckpointRoute' AS route,
           string_agg(id::text, ',' ORDER BY id) AS ids
    FROM runs
    WHERE status IN (0, 1, 5) AND coalesce(nullif(request->'ReplicationStream'->>'ID', ''), nullif(request->>'ReplicationStreamID', '')) IS NOT NULL
    GROUP BY pipeline_id, request->>'CheckpointRoute'
    HAVING count(*) > 1
  ) conflicts;
  IF route_conflicts IS NOT NULL THEN
    RAISE EXCEPTION 'replication route overlap: %', route_conflicts
      USING HINT = 'Resolve the listed active runs after confirming worker termination, then retry migration. No run was changed.';
  END IF;
  IF EXISTS (SELECT 1 FROM runs WHERE coalesce(nullif(request->'ReplicationStream'->>'ID', ''), nullif(request->>'ReplicationStreamID', '')) IS NOT NULL
             AND (pipeline_id IS NULL OR nullif(request->>'CheckpointRoute', '') IS NULL)) THEN
    RAISE EXCEPTION 'replication runs have missing pipeline or CheckpointRoute'
      USING HINT = 'Repair the historical request identity before retrying migration.';
  END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE runs ADD CONSTRAINT runs_replication_route_identity_check
  CHECK (coalesce(nullif(request->'ReplicationStream'->>'ID', ''), nullif(request->>'ReplicationStreamID', '')) IS NULL OR
         (pipeline_id IS NOT NULL AND nullif(request->>'CheckpointRoute', '') IS NOT NULL));
DROP INDEX runs_active_replication_route_idx;
CREATE UNIQUE INDEX runs_active_replication_route_idx
  ON runs (pipeline_id, (request->>'CheckpointRoute'))
  WHERE status IN (0, 1, 5) AND coalesce(nullif(request->'ReplicationStream'->>'ID', ''), nullif(request->>'ReplicationStreamID', '')) IS NOT NULL;

-- +goose Down
-- Application rollback must retain this correctness fence.
-- +goose StatementBegin
DO $$ BEGIN
  RAISE EXCEPTION 'stream admission repair cannot be downgraded; retain the fence on application rollback';
END $$;
-- +goose StatementEnd
