-- name: GetWebhookEndpointTenant :one
SELECT tenant_id FROM webhook_endpoints WHERE source_connection_id = @source_connection_id;

-- name: GetWebhookEndpointBySource :one
SELECT e.source_connection_id, e.tenant_id, e.config_revision, e.enabled,
  e.verifier, e.verifier_params, e.ingress, e.secret_refs, e.previous_key_expires_at, e.resources,
  e.success_status, e.max_body_bytes, e.pipeline_id, e.route_key,
  rs.desired_state AS route_desired_state, rs.run_spec,
  EXISTS (
    SELECT 1 FROM replication_streams s
    JOIN pipelines sp ON sp.id = s.pipeline_id AND sp.tenant_id = s.tenant_id AND sp.is_deleted = 0
    WHERE s.source_connection_id = e.source_connection_id AND s.tenant_id = e.tenant_id
      AND s.status = 0
  ) AS accepting
FROM webhook_endpoints e
JOIN connections c ON c.id = e.source_connection_id AND c.tenant_id = e.tenant_id
LEFT JOIN pipelines p ON p.id = e.pipeline_id AND p.tenant_id = e.tenant_id
  AND p.is_deleted = 0
LEFT JOIN replication_streams rs ON rs.pipeline_id = e.pipeline_id
  AND rs.route_key = e.route_key AND rs.tenant_id = e.tenant_id
  AND rs.source_connection_id = e.source_connection_id
  AND rs.status = 0 AND p.id IS NOT NULL
WHERE e.tenant_id = @tenant_id AND e.source_connection_id = @source_connection_id
  AND c.kind = 'source' AND c.is_deleted = 0;

-- name: InsertWebhookEndpoint :execrows
INSERT INTO webhook_endpoints (
  source_connection_id, tenant_id, verifier, verifier_params, ingress, pipeline_id, route_key, enabled,
  secret_refs, previous_key_expires_at, resources, success_status, max_body_bytes
) VALUES (
  @source_connection_id, @tenant_id, @verifier, @verifier_params, @ingress, @pipeline_id, @route_key, @enabled,
  @secret_refs, @previous_key_expires_at, @resources, @success_status, @max_body_bytes
)
ON CONFLICT (source_connection_id) DO NOTHING;

-- name: UpdateWebhookEndpoint :execrows
UPDATE webhook_endpoints SET config_revision = @next_revision,
  updated_at = @updated_at, verifier = @verifier, verifier_params = @verifier_params, ingress = @ingress, pipeline_id = @pipeline_id,
  route_key = @route_key, enabled = @enabled, secret_refs = @secret_refs,
  previous_key_expires_at = @previous_key_expires_at, resources = @resources,
  success_status = @success_status, max_body_bytes = @max_body_bytes
WHERE source_connection_id = @source_connection_id AND tenant_id = @tenant_id
  AND config_revision = @expected_revision;

-- name: ListWebhookEndpointVerifications :many
-- The verification each live endpoint copied from its source connector, so
-- the server can bring it up to date with the connector's current spec.
SELECT e.source_connection_id, e.tenant_id, e.config_revision, c.connector,
  e.verifier, e.verifier_params, e.ingress
FROM webhook_endpoints e
JOIN connections c ON c.id = e.source_connection_id AND c.tenant_id = e.tenant_id
WHERE c.kind = 'source' AND c.is_deleted = 0;

-- name: UpdateWebhookEndpointVerification :execrows
UPDATE webhook_endpoints SET config_revision = config_revision + 1,
  updated_at = @updated_at, verifier = @verifier, verifier_params = @verifier_params, ingress = @ingress
WHERE source_connection_id = @source_connection_id AND tenant_id = @tenant_id
  AND config_revision = @expected_revision;

-- name: ListWebhookReleases :many
SELECT e.source_connection_id, e.tenant_id, c.is_deleted AS deleted,
  s.id AS stream_id, se.positions
FROM webhook_endpoints e
JOIN connections c ON c.id = e.source_connection_id AND c.tenant_id = e.tenant_id
LEFT JOIN replication_streams s ON s.source_connection_id = e.source_connection_id
  AND s.tenant_id = e.tenant_id AND s.status = 0
  AND EXISTS (
    SELECT 1 FROM pipelines p WHERE p.id = s.pipeline_id
      AND p.tenant_id = s.tenant_id AND p.is_deleted = 0
  )
LEFT JOIN stream_epochs se ON se.stream_id = s.id AND se.epoch = s.last_epoch AND s.last_epoch > 0;

-- name: DeleteWebhookEndpointsOfDeletedSources :execrows
DELETE FROM webhook_endpoints
WHERE source_connection_id IN (
  SELECT e.source_connection_id FROM webhook_endpoints e
  JOIN connections c ON c.id = e.source_connection_id AND c.tenant_id = e.tenant_id
  WHERE c.is_deleted = 1 AND c.deleted_at < @deleted_before
);
