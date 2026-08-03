# Known traps

Validation traps, in the order you will hit them:

- **Unknown keys fail twice.** The JSON Schema is closed (`additionalProperties: false` everywhere) and the YAML decode is strict. A typo'd key is an error, never silently ignored.
- **`fields` is a map.** List syntax (`- name: id`) is explicitly rejected. Map order is preserved and becomes column order.
- **Auth and pagination are one-key maps.** `auth:` and `pagination:` each take exactly one strategy key. Two keys, or an unknown strategy name, is an error.
- **oauth2 is client-credentials only.** There is no authorization-code or refresh-token support. If the API only offers delegated OAuth, the system fails eligibility — do not fake it with a static bearer that will expire.
- **Auth template scopes are narrow.** Auth params may reference `config`, `state`, `env` only. `parent.*` or `cursor.*` in auth fails at parse time by design.
- **offset pagination targets must match.** `offset:` and `limit:` must both be `query.*` or both `body.*`. Mixing targets is an error, as is `header`.
- **`primary_key` entries must be declared fields** (when `fields` is non-empty).
- **`mode: remainder` and `shape:` are json-only** and cannot be combined on one field.
- **Parent cycles are rejected.** `for_each` chains must be acyclic.
- **Checkpoint keys must be unique.** Two resources with the same `incremental.checkpoint_key` (or `cursor_field` fallback) collide and are rejected.
- **Enum lists live in two places.** `grammar.v1.json` and `grammar_enums.go` are manually synced mirrors. If you ever extend the grammar, change both (and the `AuthSpec`/`PaginationSpec` unmarshalers in `manifest.go` for concise strategies).

Idioms per shipped manifest — copy the closest match:

| Manifest | Use it as the example for |
|---|---|
| `github.yaml` | Link-header pagination in `defaults:`, `field_sets` + `use_fields`, two-level `for_each` with `capture`, path `params` |
| `linear.yaml` | GraphQL (every resource is `POST /graphql` with `body.template.query`), cursor in `body.variables.after`, `shape:` projections of nested json |
| `slack.yaml` | `incremental:`, 200-wrapped error envelopes, `cardinality: one` singletons, cursor in query, non-secret config with defaults |
| `attio.yaml` | offset pagination in the body, POST `/query` endpoints, uuid keys, large resource catalogs |
| `notion.yaml` | dynamic discovery with scope injection, mixed cursor injection (query vs body per resource), 3-level nesting, `include:` |

Research traps:

- Verify pagination against the API's actual response examples, not its prose. The cursor `response:` path must match the real payload byte-for-byte.
- Check whether page-size maximums differ per endpoint; set `page_size` to the documented max.
- Timestamps: use `timestamptz` for ISO-8601 with offset, `timestamp` only for naive datetimes, `string` for epoch-as-string cursor fields you also declare in `incremental` with `comparator: numeric` (Slack's `ts` pattern).
- Nullable is the common case. Any field the docs don't mark required should be `type?`.
