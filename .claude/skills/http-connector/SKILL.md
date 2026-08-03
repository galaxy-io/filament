---
name: http-connector
description: Research an HTTP API and build a fully wired filament connector (manifest + catalog wiring + docs + test). Takes one or more system names as arguments; multiple systems fan out to a swarm. Use when asked to add, build, or generate an HTTP/SaaS source connector.
---

# Build an HTTP connector

Arguments are system names, e.g. `/http-connector zendesk` or `/http-connector zendesk hubspot greenhouse`. One system runs inline below. More than one runs the swarm (last section).

Each connector is a YAML manifest plus mechanical wiring. Read `references/grammar-guide.md` before authoring, `references/pitfalls.md` before validating, `references/wiring.md` before wiring. The authoritative grammar is `connectors/http/manifest/grammar.v1.json` and the package doc comment at the top of `connectors/http/manifest/manifest.go`. The five shipped manifests in `connectors/http/manifests/` are worked examples.

## 1. Eligibility check

Before writing anything, confirm the API is expressible in grammar v1:

- REST or GraphQL over HTTP with JSON responses.
- Auth is one of: none, static bearer token, static header, HTTP basic, or OAuth2 **client credentials**. Authorization-code and refresh-token flows are NOT supported. APIs that only offer user-delegated OAuth fail eligibility.
- Pagination is one of: cursor (in query, body, or header), offset/limit, page number, `Link` response header, or a full next-page URL in the response. Fully custom schemes fail.

If ineligible, stop and report exactly which requirement fails and what the API offers instead. Do not force a partial connector.

## 2. Research

Use WebSearch/WebFetch against the official API docs. Produce, before authoring:

- Base URL, required headers (API version pins, Accept), auth mechanism and what config the user must supply.
- Rate limits, including dynamic rate-limit response headers if documented.
- Exact pagination mechanics: request param name and location, response path of the cursor/next link, has-more indicator, max page size.
- The core system-of-record resources: endpoint paths, HTTP method, response envelope shape (where the records array lives), stable primary key, important typed fields with their JSON paths, parent/child nesting.
- Incremental candidates: an updated/created timestamp or monotonic cursor, and whether the API accepts a start parameter for it.
- Error envelope: does the API return errors inside 200 responses (Slack-style `ok: false`)?

Prefer fewer resources done correctly over exhaustive coverage. Target the entities that hold record data (tickets, contacts, deals, candidates), not admin/settings endpoints.

## 3. Author the manifest

Write `connectors/http/manifests/<name>.yaml` using the concise syntax described in `references/grammar-guide.md`. Conventions from the shipped manifests:

- Every resource gets `primary_key` and a trailing catch-all column: `raw: { path: $, type: json, mode: remainder }`.
- Shared request settings (method, page-size query, response envelope, pagination) go in `defaults:`; shared field bundles go in `field_sets:`.
- Secrets are `type: secret` config entries referenced as `config.<key>` in auth.
- Declare `incremental:` when the API supports a start parameter for a cursor field.
- End with `discovery: { mode: static }` unless the API has selectable containers (channels, databases, repos) worth surfacing individually.

## 4. Validate

Loop until clean:

```
go test ./connectors/http -run TestManifests
```

The failure output aggregates every grammar and semantic error with its YAML path. Fix all of them, not just the first.

## 5. Wire

Follow `references/wiring.md` exactly: embed + constructor in `connectors/http/catalog.go`, registration line in `connectors/http/register.go`, docs page `docs/pages/connectors/sources/<name>.mdx` plus a card in `http.mdx`, and an httptest-backed test in `connectors/http/source_test.go`.

## 6. Verify

```
go test ./connectors/http/...
go vet ./connectors/http/...
go build -o /dev/null ./connectors/http
```

Never leave a compiled binary in the tree. Never commit or branch — the user runs git themselves. Finish by reporting per-system: resources covered, auth/pagination choices, incremental support, and anything that needs a live-credential smoke test.

## Swarm mode (2+ systems)

Orchestrate with the Workflow tool. Stages, pipelined per system with no cross-system barrier:

1. **Research** — one agent per system executes step 2 and returns a structured spec (auth, pagination, rate limits, resource list with paths/keys/fields, incremental candidates, eligibility verdict). Ineligible systems drop out with the reason; they still appear in the final report.
2. **Author + validate** — one agent per system executes steps 3–4 and writes only files that system exclusively owns: its manifest, its docs page, its test functions. It must NOT touch `catalog.go`, `register.go`, or `http.mdx`. No worktree isolation needed because owned files never overlap.
3. **Review** — one adversarial agent per manifest re-checks the manifest against the API docs: pagination param names and response paths exact, field types match documented payloads, primary key actually unique, incremental comparator correct. Findings go back to the author agent (or get fixed directly) before the final stage.
4. **Finalize** — a single agent, after all systems land, edits the shared files once (`catalog.go`, `register.go`, `http.mdx` cards), runs the full step-6 verification, and emits the per-system report.

Each subagent prompt must point at this skill directory so the agent reads the same references.
