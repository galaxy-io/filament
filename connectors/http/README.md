# HTTP manifests

Bundled connectors live at `manifests/<name>/<api-version>/manifest.yaml`.
Their `registry.json` selects the default upstream API version and maturity.
The manifest grammar is `manifest/grammar.v1.json`.

## Splitting a manifest

A root manifest can include resource and field-set files.

```yaml
version: 1
name: example
display_name: Example
description: Example records.
dark_logo_url: https://example.com/dark.svg
light_logo_url: https://example.com/light.svg
connection:
  base_url: https://api.example.com
includes:
  - fields/common.yaml
  - resources/items.yaml
```

`fields/common.yaml` can declare reusable fields.

```yaml
field_sets:
  identity:
    id: string
    updated_at: timestamptz
```

`resources/items.yaml` declares ordinary resources using the same grammar as
an inline `resources` list.

```yaml
resources:
  - name: items
    path: /items
    primary_key: [id]
    use_fields: [identity]
    fields:
      raw: {path: $, type: json, mode: remainder}
    response:
      records: $.items
```

Fragments may contain only `resources`, `field_sets`, and further `includes`.
Keep configuration, authentication, defaults, discovery, and metadata in the root.
Includes are relative to their containing file and must stay within the root
manifest directory. Remote URLs, included symlinks, cycles, repeated inclusions,
and duplicate resource or field-set names are rejected. YAML aliases do not
cross file boundaries. The loader limits nesting to 16 levels, files to 128,
and combined input to 32 MiB.

Root declarations come first, followed by includes in declaration order. Field
order is preserved. The first eligible resource is the connection probe, so list
that resource first. Includes never override existing declarations.

`manifest.Load` supports local files. `manifest.LoadFS` supports embedded or other
filesystems. `manifest.ReadFS` returns the expanded YAML for applications that
need immutable bytes. `manifest.Parse` and `NewManifest` accept expanded bytes
and reject unresolved includes. Catalog loading expands includes automatically
and validates the resulting manifest before registration. Both single-file and
composed manifests use the same runtime.

## Shared field definitions

`field_sets` define reusable ordered groups. A resource's `use_fields` list
concatenates those groups in order, followed by its own `fields`. Use
`exclude_fields` to omit inherited fields. Duplicate names are rejected.

`field_overrides` replaces an existing field in its original position. Each
override is a complete field declaration, so omitted properties use their normal
defaults. Unknown names are rejected. For example, a resource can require an
otherwise nullable timestamp without repeating the surrounding shared fields.

```yaml
use_fields: [identity]
field_overrides:
  updated_at: {path: MetaData.LastUpdatedTime, type: timestamptz}
fields:
  total: decimal
```

Expansion happens when loading the manifest, so schemas and runtime projections
retain their ordinary flat representation.

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

## Response validation and field projections

- `response.strict_records` rejects missing or null record paths and nonobject
  array entries. `empty_object` allows a missing array when the named envelope
  is an empty object. `required_objects` validates surrounding objects.
- `response.match` checks record fields against config or parent templates.
  `response.unwrap` selects exactly one object from `variants`, writes its name
  to `tag`, and retains other wrapper fields under `remainder`. Conflicts fail.
- Field `value` renders a config or parent template. String field `tuple` encodes
  a collision-free JSON array of scalar record paths. `literal:` entries are
  constants and `query:` entries encode the named nonempty query parameters.
  Missing tuple components become empty strings, so identity components should
  also be projected as required fields when they must exist.
- Decimal fields can declare `precision` and `scale` for exact decimal128 output.
  Out-of-range or overprecise values fail instead of being rounded. Existing
  decimal declarations without precision keep their text representation.

## Request configuration and resource selection

- `connection.base_url_field` selects a key from `connection.base_urls`. This
  supports fixed environment endpoints while retaining an existing config field.
- Config fields support `pattern`, `format: date`, integer `minimum` and `maximum`,
  `requires`, and date-field `before` constraints. Resources can require extra
  fields through `required_config` when selected.
- Offset pagination supports `start`, `page_size_from`, and `state` targets.
  For example, `offset: state.offset` and `limit: state.page_size` expose numeric
  values to request templates instead of adding separate query parameters.
- `for_each_config` makes one read per distinct nonempty comma-separated config
  value. The current value is available as `parent.<config_field>`. This is for
  top-level full-read resources. It does not create a discoverable parent resource.
- `omit_empty_query` omits empty optional query values. Existing manifests retain
  their behavior unless this is enabled.

## Completing and resuming scans

- Incremental `inject_into: state` exposes the fixed lower bound under
  `state.<start_param>`. `value_prefix` and `value_suffix` surround the wire value.
  `commit_on_complete` withholds the new watermark until the resource finishes.
  Limited or failed scans replay from the previous committed watermark.
- `restart_on_resume` disables offset resume and rejects repeated primary keys
  within a scan. It is appropriate for collections without stable offset cursors.
