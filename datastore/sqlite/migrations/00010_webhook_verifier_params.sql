-- +goose Up
ALTER TABLE webhook_endpoints
  ADD COLUMN verifier_params TEXT NOT NULL DEFAULT '{}'
  CHECK (json_valid(verifier_params) AND json_type(verifier_params) = 'object');
ALTER TABLE webhook_endpoints
  ADD COLUMN ingress TEXT NOT NULL DEFAULT '{}'
  CHECK (json_valid(ingress) AND json_type(ingress) = 'object');

-- +goose Down
ALTER TABLE webhook_endpoints DROP COLUMN ingress;
ALTER TABLE webhook_endpoints DROP COLUMN verifier_params;
