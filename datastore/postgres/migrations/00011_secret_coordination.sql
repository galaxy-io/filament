-- +goose Up
CREATE TABLE secret_coordination (
    tenant_id UUID NOT NULL REFERENCES tenants (id),
    scope TEXT NOT NULL,
    ref TEXT NOT NULL,
    owner TEXT NOT NULL DEFAULT '',
    expires_at BIGINT NOT NULL DEFAULT 0,
    version TEXT NOT NULL DEFAULT '',
    pending BOOLEAN NOT NULL DEFAULT false,
    deleted BOOLEAN NOT NULL DEFAULT false,
    PRIMARY KEY (tenant_id, scope, ref)
);

-- +goose Down
DROP TABLE secret_coordination;
