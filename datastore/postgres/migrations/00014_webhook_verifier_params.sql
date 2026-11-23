-- +goose Up
ALTER TABLE webhook_endpoints
  ADD COLUMN verifier_params JSONB NOT NULL DEFAULT '{}'::jsonb
  CHECK (jsonb_typeof(verifier_params) = 'object'),
  ADD COLUMN ingress JSONB NOT NULL DEFAULT '{}'::jsonb
  CHECK (jsonb_typeof(ingress) = 'object');

-- +goose Down
ALTER TABLE webhook_endpoints DROP COLUMN ingress, DROP COLUMN verifier_params;
