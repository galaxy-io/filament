# Manifest grammar v1, distilled

Authoritative sources: `connectors/http/manifest/grammar.v1.json` (shape) and the doc comment at the top of `connectors/http/manifest/manifest.go` (semantics). This file is the working summary.

Parse pipeline: JSON Schema over the raw YAML → strict decode (unknown keys are errors) → `version` must be `1` → Normalize (defaults, desugaring) → semantic validation (all errors aggregated). The concise YAML you write is not the runtime model; Normalize rewrites it.

## Top level

```yaml
version: 1            # required, literally 1
name: <slug>          # required
config: {}            # user-facing connector settings
defaults: {}          # inherited by every resource
field_sets: {}        # reusable field bundles
connection: {}        # required
resources: []         # required, at least one
discovery: {}         # static or dynamic
```

## config

Each entry becomes a field in the connector's UI config schema:

```yaml
config:
  api_token: { type: secret, required: true, help: API token from Settings > API }
  subdomain: { type: string, required: true, help: Account subdomain }
  region:    { type: enum, enum: [us, eu], default: us }
```

Types: `string | int | bool | duration | enum | object | secret`. Optional `scope: connection | pipeline`.

## connection

```yaml
connection:
  base_url: https://api.example.com     # required, a URI
  auth: ...                             # one strategy, see below
  headers: { Accept: application/json } # sent on every request
  timeout_seconds: 60
  rate_limit:
    requests_per_second: 5
    dynamic:                            # optional, header-driven
      remaining_header: X-RateLimit-Remaining
      reset_header: X-RateLimit-Reset
      reset_format: unix_seconds        # unix_seconds | seconds_from_now | http_date
      min_floor_rps: 0.5
```

### auth — exactly one strategy key

```yaml
auth: { bearer: config.token }                       # Authorization: Bearer <token>
auth: { header: { name: X-Api-Key, value: config.key } }
auth: { basic: { username: config.email, password: config.api_token } }
auth: { basic: { username: config.api_key } }        # password optional (Stripe-style)
auth:
  oauth2:                                            # client credentials ONLY
    token_url: https://example.com/oauth/token
    client_id: config.client_id
    client_secret: config.client_secret
    scope: read
auth: none
```

Bare dotted references (`config.token`) auto-wrap to `{{ config.token }}`. Auth templates may reference only `config`, `state`, `env` — never `parent` or `cursor`.

## defaults

Inherited by every resource; resource-local values win. `headers`/`query` merge per key.

```yaml
defaults:
  method: GET
  query: { per_page: "100" }
  response:
    records: $              # or $.data.items etc.
    pagination: { link: next }
```

## field_sets / use_fields

```yaml
field_sets:
  lifecycle: { created_at: timestamptz, updated_at: "timestamptz?" }
resources:
  - name: tickets
    use_fields: [lifecycle]      # prepends the bundle's fields
    exclude_fields: [created_at] # drops by name after expansion
```

## resources

```yaml
- name: tickets            # required, unique
  path: /v2/tickets        # required
  method: GET              # default from defaults.method
  query: { sort: created_at }
  headers: {}
  primary_key: [id]        # entries must be declared field names
  records: $.tickets       # $ = response root is the array; $.a.b = array at path a.b
  fields: ...              # see below
  pagination: ...          # see below
  incremental: ...         # optional
  emit_as: other_name      # optional output stream rename
```

### fields — a map, never a list

```yaml
fields:
  id: int64                                   # shorthand: name is both column and JSON path
  subject: string
  description: string?                        # trailing ? = nullable
  assignee_id: int64?
  status: { path: status, type: string }      # expanded form
  org: { path: organization.name, type: string, nullable: true }
  raw: { path: $, type: json, mode: remainder }   # catch-all: everything not mapped above
```

Types: `string bool int16 int32 int64 float32 float64 decimal date time timestamp timestamptz json uuid`. `mode: remainder` and `shape:` (sub-object projection, see linear.yaml) are json-only and mutually exclusive.

### Parent/child fan-out

```yaml
- name: repositories
  path: /orgs/{organization}/repos
  params: { organization: config.organization }   # substitutes {organization} in path
  capture: { repository: name }                    # exposes parent.repository to children
- name: issues
  path: /repos/{organization}/{repository}/issues
  params:
    organization: config.organization
    repository: parent.repository
  for_each: repositories                           # one run per parent record
  fields:
    repository: { path: parent.repository, type: string }  # denormalize the parent key
```

### pagination — one strategy key (or inherit from defaults)

```yaml
pagination: { link: next }                # Link response header, rel="next"
pagination: { next_url: $.paging.next }   # full next-page URL in the response
pagination:
  cursor:
    response: response_metadata.next_cursor   # where the cursor appears in the response
    request: query.cursor                      # query.<name> | body.<path> | header.<name>
    more: has_more                             # optional has-more path
    null_terminates: true                      # optional
pagination:
  offset: { offset: body.offset, limit: body.limit, page_size: 500 }  # both must share query.* or body.*
pagination:
  page: { number: query.page, size: query.per_page, page_size: 100, total_pages: $.meta.total_pages }
pagination: none
```

### incremental

```yaml
incremental:
  cursor_field: updated_at      # field whose max value is checkpointed
  start_param: updated_since    # request param for the next run's floor
  inject_into: query            # query | body | header (all three keys above required)
  comparator: time              # lex | numeric | time
  overlap_seconds: 60           # optional re-read window
```

See slack.yaml for the worked example (`ts` + `oldest` + `numeric`).

### Error envelopes in 200 responses

```yaml
response:
  error: { path: error, when_present: true, message_path: error }
```

See slack.yaml (`ok: false` responses).

## discovery

```yaml
discovery: { mode: static }                       # extract every declared resource
discovery: { mode: static, include: [a, b] }      # subset
discovery:                                        # dynamic: surface selectable items
  mode: dynamic
  resources:
    - from: databases                             # a declared resource that lists containers
      map: { kind: database, id: $.id, name_paths: ["$.title"], default_enabled: "true" }
      scope: { field: database_id, inject_into: body, applies_to: [pages] }
```

See notion.yaml (dynamic) and slack.yaml (channel discovery) for full examples.
