# Correctness traps and worked examples

Use this reference when reviewing a resource design and before final validation.
Consult only examples relevant to the API, and verify their current contents.

## Choose the nearest working pattern

All manifest filenames below are under `connectors/http/manifests/`.

| Manifest | Useful patterns |
|---|---|
| `github.yaml` | Link pagination, shared fields, path parameters, nested reads, provider throttle signals |
| `linear.yaml` | GraphQL bodies and variables, body cursors, shaped JSON projections |
| `slack.yaml` | 200-wrapped errors, singleton responses, list config, child activity gating, numeric timestamps |
| `attio.yaml` | POST query reads and body offset pagination |
| `notion.yaml` | Nested content and parent/child reads |
| `posthog.yaml` | Configurable host, project-scoped reads, provider-specific query behavior |
| `stripe.yaml` | Basic auth with optional password, numeric event cursors |
| `granola.yaml` | Separate paginated transcripts, keyless repeated items, update-based note reads |
| `gong.yaml` | Account base URL, bulk POST reads, full-only data, exact empty-response rules, folder JSON arrays |

These are implementation examples, not evidence for another company's API.
Do not copy endpoint limits, auth scopes, headers, or timing assumptions across
providers. Avoid fixed counts of catalog connectors in guidance or docs.

## Record identity and completeness

- Define one row before choosing its key. An ID may only be unique within a
  parent. Include the parent key when that is the documented identity.
- Never manufacture uniqueness from speaker, text, timestamps, or list position.
  Transcript items and repeated snippets can legitimately coincide. Preserve
  them in a JSON array on a keyed parent, or use a keyless resource with full
  replacement when that matches the API's row structure.
- Keep opaque identifiers as strings. The shared decoder preserves JSON numbers
  through `json.Number`. Test long IDs through projection, captures, nested JSON,
  and raw remainder. Do not reintroduce float64 decoding in an intermediate step.
- Treat fields absent from valid responses as nullable. Distinguish null from an
  empty array or object. Required identity and incremental fields need actual
  guarantees, not merely examples where they happen to be present.
- Use JSON for useful nested structures instead of projecting a few leaves and
  losing the rest. A trailing `raw` remainder preserves unmodeled fields without
  duplicating every projected field.
- Full upsert refreshes keyed rows but does not remove absent ones. Full replace
  can reflect removals only within the upstream account's visibility and retention.

## Requests, selection, and recovery

- Defaults can leak query parameters or pagination into singleton and child
  endpoints. Put only shared settings there and explicitly use `pagination: none`
  where needed. Recheck body injection and filters on the second page.
- Resource defaults affect initial selection, not access control. Keep optional
  resources discoverable. Selected resources with missing access should fail
  clearly, not vanish from the result.
- An unselected parent may still need to be fetched for selected children. It
  should not be emitted. Use `capture_only` for an internal dependency walk,
  not to hide a useful user-facing resource.
- Cursor recovery is not incremental replication. Top-level reads can resume a
  stored page cursor. The current engine restarts from the beginning if the first
  resumed fetch fails. Child reads restart. Do not document different guarantees.
- Filtering parents by their own update time can miss changed children. A child's
  activity timestamp may need a separate full parent walk. Even with gating,
  child watermarks are shared across parents, not maintained per parent.
- Do not request expiring media URLs or binary downloads by default just because
  the API supports them. Check whether durable data is returned, whether extra
  scope is required, and whether downloads fit the source's JSON model.

## Errors and runtime changes

The status and body jointly determine an empty-result exception. A generic
404 suppression can hide an invalid host, endpoint, parent, or workspace.
`response.empty` is resource-local and opt-in. It matches an entire string array,
not a substring. Its exact syntax is in [grammar-guide.md](grammar-guide.md).
Unverified errors stay errors, and API message changes require renewed evidence.

Before changing shared behavior, check whether a documented request shape or an
existing primitive solves the problem. If a runtime fix is necessary, keep the
unconfigured behavior intact. Test failure paths and an unaffected manifest.
Do not add provider-name branches, permissive fallbacks, arbitrary limits, or
new configuration without a concrete requirement.

A grammar change may involve JSON Schema, typed fields and YAML decoding,
normalization, semantic validation, runtime code, tests, and author docs.
Update `grammar_enums.go` only when changing an enum it defines. Concise auth and
pagination strategies also have custom unmarshalers. Adding a field does not
automatically require changing every one of these files.

## Validation traps

- Unknown properties and invalid strategy shapes fail validation. `fields` is a
  mapping, not a list. Auth and pagination use one strategy key.
- Resource keys must refer to projected fields. Incremental cursor fields must
  be projected, non-nullable, and compatible with their comparator.
- `mode: remainder` and `shape` are JSON-only and mutually exclusive.
- Parent chains must be acyclic. Checkpoint keys must be unique. Pagination and
  incremental injection must not target the same request field.
- Existing mocks are examples, not live API proof. Build new fixtures from the
  researched response contract and include cases that would break an incorrect
  implementation.
- Top-level resources can fetch concurrently. Protect mock-server counters and
  maps with mutexes or atomics. Do not assume extraction `Parallelism: 1` makes
  HTTP handlers serial. Use the race detector when testing shared state.
