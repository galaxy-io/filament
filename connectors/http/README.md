# HTTP manifests

Bundled connectors live at `manifests/<name>/<api-version>/manifest.yaml`.
Their `registry.json` selects the default upstream API version and maturity.
The manifest grammar is `manifest/grammar.v1.json`.

## Managed OAuth refresh

The `oauth2` strategy accepts `grant_type: refresh_token`,
`client_auth: client_secret_basic`, and `refresh_token: config.refresh_token`.
Use `managed_refresh: true` for providers that rotate refresh tokens. Supply a
`binding` list of literal or `config` template components identifying the provider,
account, environment, and OAuth client. Do not include the refresh token itself.
The list identifies the authorization settings for the run.

Managed refresh uses the runner's configured secret manager, including PostgreSQL,
SQLite, AWS Secrets Manager, and Google Secret Manager. Access tokens remain in
run-scoped memory. Only changed refresh tokens are written to the same secret.
Refreshes serialize within the run without datastore leases.
Without a bound store, extraction fails before exchanging tokens. Connection
validation checks configuration only when no store is bound. Direct Go callers can supply
an `oauth.Store`. A rejected access token triggers one refresh and
one replay. A second HTTP 401 fails the request.

An optional `fallback_token: config.access_token` allows manual access-token
connections when all refresh credentials are absent. Partial refresh credentials
are rejected. Initial browser authorization remains external to Filament.
