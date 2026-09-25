# Manifest grammar v1, distilled

Authoritative sources: `connectors/http/manifest/grammar.v1.json` (shape),
`manifest.go` (decoding), `validate.go` (normalization and semantics), and the
runtime packages. Comments and this summary can lag behind the implementation.

Parse pipeline: JSON Schema over the raw YAML → strict decode (unknown keys are errors) → `version` must be `1` → Normalize (defaults, desugaring) → semantic validation (all errors aggregated). The concise YAML you write is not the runtime model; Normalize rewrites it.

Except for the complete minimal example, code blocks below show fragments or
alternative strategies. Do not concatenate them into a single manifest.

## Top level

```yaml
version: 1            # required, literally 1
name: <slug>          # required
display_name: Example # required catalog metadata
description: Read records from Example.
dark_logo_url: https://cdn.example.com/example-dark.svg
light_logo_url: https://cdn.example.com/example-light.svg
api_version: "v1"     # upstream API version; omit when the vendor does not version
config: {}            # user-facing connector settings
defaults: {}          # inherited by every resource
field_sets: {}        # reusable field bundles
connection: {}        # required
resources: []         # required, at least one
discovery: {}         # static or dynamic
```

Metadata belongs in the manifest, not in constructor arguments. Resolve real
logo URLs using [wiring.md](wiring.md).

`api_version` is the version the manifest was researched against: a pinned
header value, a path segment such as `v2`, or a GraphQL schema date. It must
equal any pinned version header (`Stripe-Version`, `Notion-Version`,
`X-GitHub-Api-Version`, `API-Version`). Omit it for unversioned APIs.

## Complete minimal example

This synthetic API contract demonstrates valid syntax. Research the actual
provider before adapting paths, limits, or response fields.

```yaml
version: 1
name: example
display_name: Example
description: Records from Example.
dark_logo_url: https://cdn.example.com/example-dark.svg
light_logo_url: https://cdn.example.com/example-light.svg
api_version: "v1"
config:
  api_key: { type: secret, required: true, help: Example API key }
connection:
  base_url: https://api.example.com
  auth: { bearer: config.api_key }
  headers: { Accept: application/json }
  timeout_seconds: 60
resources:
  - name: items
    path: /v1/items
    method: GET
    primary_key: [id]
    fields:
      id: string
      title: string?
      raw: { path: $, type: json, mode: remainder }
    response:
      records: $.items
      pagination:
        cursor:
          response: next_cursor
          request: query.cursor
          null_terminates: true
discovery:
  mode: static
  default_resources: [items]
```

## config

Each entry becomes a field in the connector's UI config schema:

```yaml
config:
  api_token: { type: secret, required: true, help: API token from Settings > API }
  subdomain: { type: string, required: true, help: Account subdomain }
  region:    { type: enum, enum: [us, eu], default: us }
```

Types: `string | int | bool | duration | enum | object | secret | list`.
Optional `scope: connection | pipeline`. A `list` declares its allowed values
in `enum` and uses an array for `default`. See Slack's `conversation_types`.

## connection

```yaml
connection:
  base_url: https://api.example.com     # required; templatable (see below)
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

`base_url` accepts a template and is rendered once at Configure, so it carries
auth's scope set (`config`, `state`, `env` — never `parent`/`cursor`). Use it
for APIs with regional or per-tenant hosts rather than pinning one:

```yaml
config:
  host: { type: string, default: https://us.posthog.com }
connection:
  base_url: "{{ config.host }}"
```

See posthog.yaml. Literal base URLs are unaffected. Note that tests for such a
manifest set the host through config instead of the usual
`strings.Replace(manifest, "https://api.example.com", api.URL, 1)` swap.

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
auth: { none: {} }
```

Bare dotted references (`config.token`) auto-wrap to `{{ config.token }}`. Auth templates may reference only `config`, `state`, `env` — never `parent` or `cursor`.

## defaults

Resource-local values override inherited defaults. `headers`/`query` merge per
key, so a child can accidentally inherit a list endpoint's query parameters.
Keep only genuinely shared settings here. `response.empty` is resource-local
and is rejected in defaults.

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

Use strings for opaque IDs, including long numeric identifiers. Use typed
numeric columns for quantities and `timestamptz` for timestamps with offsets.
In expanded fields, nullable is `nullable: true`, not a `?` in `type`.
`primary_key: []` is valid when rows have no stable identity. Do not manufacture
a key from text or timestamps that can repeat.

### Request bodies and singletons

```yaml
method: POST
body:
  encoding: json
  template:
    filter: {}
response:
  records: $.items
```

The body is structured YAML, not a JSON-encoded string. GraphQL uses the same
encoder with `query` and `variables` inside `template`. A mandatory empty filter
object differs from an omitted filter. JSON bodies preserve numbers and
booleans. Query and header values are strings.

```yaml
response:
  records: $
  cardinality: one
  pagination: none
```

Use this for a singleton root object, or use a nested object path in `records`.
Without `cardinality: one`, `records: $` expects an array. For an unpaginated
detail or child resource, explicitly disable any inherited pagination.

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

A parent that exists only to drive a child is `capture_only`. It is walked for
captures, never emitted or listed, and runs only when a dependent child is
selected. A child gates its fan-out with `parent.since`, which names a captured
key: empty values never fan out, and on incremental runs values below the
child's lower bound (watermark minus lookback, under the child's comparator)
are skipped. `since` requires an `incremental` block on the child.

```yaml
- name: threads
  path: /conversations.history
  for_each: conversations
  capture_only: true
  capture: { thread_ts: ts, latest_reply: latest_reply }
- name: thread_replies
  path: /conversations.replies
  query: { ts: "{{ parent.thread_ts }}" }
  for_each: threads
  parent: { since: latest_reply }                  # only threads with new replies
  incremental: { cursor_field: ts, start_param: oldest, inject_into: query, comparator: numeric }
```

A normal parent need not be capture-only just because a child depends on it.
The engine fetches required parents even when only the child is selected, and
emits only selected resources. Test that behavior and parent-scoped keys.

### pagination — one strategy key (or inherit from defaults)

```yaml
pagination: { link: next }                # Link response header, rel="next"
pagination: { next_url: paging.next }     # full next-page URL in the response (bare dot-path, no $.)
pagination:
  cursor:
    response: response_metadata.next_cursor   # where the cursor appears in the response
    request: query.cursor                      # query.<name> | body.<path> | header.<name>
    more: has_more                             # optional has-more path
    null_terminates: true                      # optional
pagination:
  offset: { offset: body.offset, limit: body.limit, page_size: 500 }  # both must share query.* or body.*
pagination:
  page: { number: query.page, size: query.per_page, page_size: 100, total_pages: meta.total_pages }
pagination: none
```

Cursor response paths use bare dot paths such as `records.cursor`, not the
`$.items` records shorthand. Missing or empty cursors terminate the walk.
Explicit null requires `null_terminates: true`. A configured `more` flag takes
precedence and a missing, null, or false flag stops pagination, so do not invent
a has-more field. Cursor pagination does not stop merely because a page is short.

Body injection changes the cursor field while preserving rendered filters and
selectors. Offset targets must both be in query or both in body. Verify page
number conventions and termination against the actual paginator before using it.
The current page strategy starts at 1 and injects into query parameters. Offset
starts at 0. Both stop on a short page, so an API that can return a short
nonterminal page does not fit these strategies without another completion signal
and corresponding runtime support.

### incremental

```yaml
incremental:
  cursor_field: updated_at      # field whose max value is checkpointed
  start_param: updated_since    # request param for the next run's floor
  inject_into: query            # query | body | header (all three keys above required)
  comparator: time              # lex | numeric | time
  checkpoint_key: tickets_updated_at # unique across resources
  overlap_seconds: 60           # optional re-read window
```

The cursor must be projected, non-nullable, and compatible with its comparator.
`initial` can set a documented first-run lower bound. `overlap_seconds` applies
to `time` or `numeric`, not `lex`. Do not collide with pagination injection.
Checkpoint keys default to the cursor field and must be unique across resources.
See Slack for `ts` + `oldest` + `numeric`, and Granola for an update timestamp.
Read [research.md](research.md) before deciding that incremental reads are sound.

### Error envelopes in 200 responses

```yaml
response:
  error: { path: error, when_present: true, message_path: error }
```

See slack.yaml (`ok: false` responses).

### Empty-result errors

The current opt-in rule matches a 4xx status and a nonempty array of strings:

```yaml
response:
  empty:
    status: 404
    body_path: errors
    body_equals: [No calls found corresponding to the provided filters]
```

The entire array must match, including case, order, and number of messages.
Invalid JSON, another status, a different body type, or extra messages do not
match. The rule belongs on an individual resource and cannot be set in defaults.
A match ends the page walk before error projection or cursor handling, keeps
prior rows, and adds no rows or watermark. Throttling handling takes precedence.

The Gong message above is an example, not a universal empty-result signature.
Verify the target endpoint's response before using the rule. Never add a blanket
ignore-404 rule or convert authentication failures into empty data. If the API
changes its message, failure is intentional until the rule can be reverified.

### Pending responses and throttling

`response.poll_pending: true` retries HTTP 202 within the existing attempt
budget. It does not implement a create-job / poll-job / download workflow.
HTTP 204 is empty. Invalid JSON in a 200 response remains an error.

HTTP 429 and server-error retries already exist. For other documented throttle
signals, `connection.rate_limit.responses` supports status plus header/body
conditions. Conditions in a rule are conjunctive. `body_contains` matches a
string case-insensitively, unlike exact `response.empty.body_equals` matching.
See GitHub's manifest and `paginate.go` before using these rules. Dynamic budget
headers and provider throttle responses do not implement a daily quota ledger.

## discovery

```yaml
discovery: { mode: static }                       # expose declared selectable resources
discovery: { mode: static, include: [a, b] }      # subset
discovery: { mode: static, default_resources: [a] } # initial selection, not visibility
discovery:                                        # dynamic: surface selectable items
  mode: dynamic
  resources:
    - from: databases                             # a declared resource that lists containers
      map: { kind: database, id: $.id, name_paths: ["$.title"], default_enabled: "true" }
      scope: { field: database_id, inject_into: body, applies_to: [pages] }
```

Static discovery is the current catalog pattern. `default_resources` omitted
selects all by default, `[]` selects none, and a named list selects those resources
without hiding the others. Capture-only resources remain hidden.

Dynamic discovery is supported but is not needed just to represent parent/child
reads. Use `connectors/http/discover_test.go` and the current grammar as examples
when users need to select upstream containers. Do not assume a named catalog
manifest still uses dynamic discovery.

The runtime also supports `mode: stream` with `ndjson`, `sse`, or `chunked_array`.
No catalog manifest currently uses it. If needed, inspect the stream schema and
implementation rather than treating a streaming endpoint as paginated JSON.
