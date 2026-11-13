-- name: GetWebhookEndpointBySource :one
SELECT e.source_connection_id, e.tenant_id, e.config_revision, e.enabled,
  e.verifier, e.secret_refs, e.previous_key_expires_at, e.resources,
  e.success_status, e.max_body_bytes, e.pipeline_id, e.route_key,
  rs.desired_state AS route_desired_state, rs.run_spec,
  EXISTS (
    SELECT 1 FROM replication_streams s
    JOIN pipelines sp ON sp.id = s.pipeline_id AND sp.tenant_id = s.tenant_id AND NOT sp.is_deleted
    WHERE s.source_connection_id = e.source_connection_id AND s.tenant_id = e.tenant_id
      AND s.status = 0 AND s.desired_state <> 'stopped'
  )::boolean AS accepting
FROM webhook_endpoints e
JOIN connections c ON c.id = e.source_connection_id AND c.tenant_id = e.tenant_id
LEFT JOIN pipelines p ON p.id = e.pipeline_id AND p.tenant_id = e.tenant_id
  AND NOT p.is_deleted
LEFT JOIN replication_streams rs ON rs.pipeline_id = e.pipeline_id
  AND rs.route_key = e.route_key AND rs.tenant_id = e.tenant_id
  AND rs.source_connection_id = e.source_connection_id
  AND rs.status = 0 AND p.id IS NOT NULL
WHERE e.tenant_id = @tenant_id AND e.source_connection_id = @source_connection_id
  AND c.kind = 'source' AND NOT c.is_deleted;

-- name: InsertWebhookEndpoint :execrows
INSERT INTO webhook_endpoints (
  source_connection_id, tenant_id, verifier, pipeline_id, route_key, enabled,
  secret_refs, previous_key_expires_at, resources, success_status, max_body_bytes
) VALUES (
  @source_connection_id, @tenant_id, @verifier, @pipeline_id, @route_key, @enabled,
  @secret_refs, @previous_key_expires_at, @resources, @success_status, @max_body_bytes
)
ON CONFLICT (source_connection_id) DO NOTHING;

-- name: UpdateWebhookEndpoint :execrows
UPDATE webhook_endpoints SET config_revision = @next_revision,
  updated_at = @updated_at, verifier = @verifier, pipeline_id = @pipeline_id,
  route_key = @route_key, enabled = @enabled, secret_refs = @secret_refs,
  previous_key_expires_at = @previous_key_expires_at, resources = @resources,
  success_status = @success_status, max_body_bytes = @max_body_bytes
WHERE source_connection_id = @source_connection_id AND tenant_id = @tenant_id
  AND config_revision = @expected_revision;

-- name: ListWebhookReleases :many
-- One row per endpoint. consumer_positions holds the latest certified
-- positions of every stream that can still resume reading the endpoint's
-- source, whether running, paused or stopped, with a JSON null for one that
-- has not certified anything yet.
SELECT e.source_connection_id, e.tenant_id, c.is_deleted AS deleted,
  COALESCE((
    SELECT jsonb_agg(COALESCE(se.positions, 'null'::jsonb))
    FROM replication_streams s
    JOIN pipelines p ON p.id = s.pipeline_id AND p.tenant_id = s.tenant_id AND NOT p.is_deleted
    LEFT JOIN stream_epochs se ON se.stream_id = s.id AND se.epoch = s.last_epoch AND s.last_epoch > 0
    WHERE s.source_connection_id = e.source_connection_id AND s.tenant_id = e.tenant_id
      AND s.status = 0
  ), '[]'::jsonb)::jsonb AS consumer_positions
FROM webhook_endpoints e
JOIN connections c ON c.id = e.source_connection_id AND c.tenant_id = e.tenant_id;

-- name: DeleteWebhookEndpointsOfDeletedSources :execrows
-- Endpoints whose source was deleted before @deleted_before. Their deliveries
-- are all older than that, so retention has already purged them.
DELETE FROM webhook_endpoints e
USING connections c
WHERE c.id = e.source_connection_id AND c.tenant_id = e.tenant_id
  AND c.is_deleted AND c.deleted_at < @deleted_before;
