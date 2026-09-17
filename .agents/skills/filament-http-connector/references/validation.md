# Validate a connector

Validation has separate jobs: the manifest must parse, requests must match the
API contract, emitted rows must be correct, and shared changes must preserve
other connectors. Passing one does not establish the others.

## Start with manifest validation

Run from the repository root after authoring the manifest:

```sh
go test ./connectors/http -run TestManifests
```

This loads every shipped YAML file through grammar validation, strict decoding,
normalization, and semantic checks. Fix all reported errors. It does not test
the upstream API, response paths, permissions, or whether reads are complete.

## Mock the API contract

Use `httptest` with fake credentials and the embedded manifest. Assert the
outgoing request as well as the resulting rows. Derive fixtures from official
schemas and sanitized observations. Add focused adversarial cases for risks in
this connector rather than a large matrix of trivial variations.

### Spec, schema, and selection

- Construct through `New<Product>()` and check metadata, required config fields,
  secret types, and missing-config rejection.
- Check exact discovery names, default selections, primary keys, and advertised
  read modes. Static discovery should not require network access.
- Verify selecting an optional resource actually reads it. For full-only
  resources, check that incremental planning is rejected.

### Requests, pages, and records

- Assert the configured host, method, path, auth, version/Accept headers, required
  query parameters, body encoding, and any content selectors.
- Cover each distinct endpoint shape and each resource's response path/key
  mapping. A single happy-path resource cannot establish a large catalog works.
- Exercise multiple pages for each pagination strategy used. Where applicable,
  include a short nonterminal page and each documented final-cursor form. Assert
  that POST filters, selectors, and GraphQL variables survive cursor injection.
- Check mapped values, missing and null optional fields, timestamps, nested
  arrays, and raw remainder. Test numeric IDs beyond float64's exact integer
  range through any affected projection, parent capture, and JSON paths.
- Preserve legitimate duplicates when the API supplies no unique key. A
  test should expose row loss if an invented key or flattened array would merge
  repeated transcript items or snippets.

### Dependencies and checkpoints

- Select a child alone and verify required parents are fetched but not emitted.
  Use more than one parent and check child state resets between them.
- Select an independent bulk resource alone and verify it does not trigger a
  redundant per-record or parent fetch.
- Where recovery matters, verify a saved top-level cursor is sent, rejected
  cursor fallback restarts the walk, and a failed restart still fails. Children
  restart under the current engine. Do not call this incremental replication.
- For incremental reads, test an existing checkpoint and exact lower-bound
  injection, lookback, ties, and watermark advancement. The lower bound must
  stay fixed across the page walk. Include late or child updates when they
  justify the design. Missing or invalid cursor values must not be skipped.

### Failures and shared runtime behavior

- Check empty success results and the provider's error envelopes, including
  errors inside HTTP 200 when relevant. Malformed JSON must fail.
- For an empty-result rule, test exact match, wrong status, unrelated body,
  extra error messages, wrong type/path, and malformed JSON. Check 401/403,
  a resource with no rule, and an unaffected manifest. A match after a good
  page must retain earlier rows without emitting the error or following its
  cursor. A connection probe must still reject invalid credentials/paths.
- Exercise throttling, Retry-After, and cancellation when adding rate-limit or
  request-loop behavior. Reuse existing shared tests where they already cover
  unchanged behavior. Do not add provider-specific retries for built-in 429s.
- Protect shared state in mock handlers. HTTP requests may be concurrent even
  when extraction options specify one worker.

## Repository checks

After focused tests pass, run:

```sh
go test ./connectors/http/...
go vet ./connectors/http/...
go build -o /dev/null ./connectors/http
```

Use `go test -race ./connectors/http/...` for concurrency or shared-state changes,
or to investigate a suspected test race. If the shared decoder or projection
changes, test existing numeric representations as well as the new provider's
IDs. If the grammar changes, test invalid declarations and default behavior,
not just the new accepted syntax.

Do not leave a compiled binary in the tree. Broaden verification only when the
changed scope, a failure, or an unresolved concern warrants it. A logo-only edit
needs asset and diff checks, not a full new integration test suite.

## Docs and assets

Check `docs/package.json` for the current scripts. From `docs`, run the existing
`validate` and `check` scripts with the repo's package manager. They currently
invoke `mint validate` and `mint broken-links`. Use the supported Node runtime
and installed dependencies. Do not upgrade packages to work around a local
runtime problem unrelated to the connector.

Check source navigation, overview entries, internal links, and both catalog logo
URLs. Separate new failures from existing unrelated ones, naming the affected
files in the report. A failed global link check is not a passing check just
because its findings are unrelated.

Inspect the final diff for unintended changes, debug fixtures, credentials,
generated artifacts, and stale claims about coverage or maturity.

## Live verification and completion

When authorized credentials and account features are available, run a bounded
read-only smoke test. Confirm the connection probe, a known record or period,
pagination, empty results, and selected optional resources. Compare IDs and
counts where the upstream view supports it. Avoid a large account backfill just
to prove connectivity, and never put credentials or customer data into fixtures.

If live access is unavailable, complete mock validation and register the new
connector as alpha. List the exact pending checks, including unavailable account
features and inferred response behavior. Do not describe mock fixtures as
captured live responses or silently omit a promised resource.

Completion means the agreed resource scope is implemented, the shared runtime
remains compatible, wiring and docs agree with the behavior, and any remaining
live uncertainty is explicit. Report outcomes rather than every command's output.
