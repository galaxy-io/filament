-- +goose Up
DROP TABLE secret_coordination;

-- +goose Down
CREATE TABLE secret_coordination (
    tenant_id TEXT NOT NULL REFERENCES tenants (id),
    scope TEXT NOT NULL,
    ref TEXT NOT NULL,
    owner TEXT NOT NULL DEFAULT '',
    expires_at INTEGER NOT NULL DEFAULT 0,
    version TEXT NOT NULL DEFAULT '',
    pending INTEGER NOT NULL DEFAULT 0,
    deleted INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (tenant_id, scope, ref)
);
