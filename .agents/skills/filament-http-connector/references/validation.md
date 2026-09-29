# Validate a connector

Validation has separate jobs: the manifest must parse, requests must match the
API contract, emitted rows must be correct, and shared changes must preserve
other connectors. Passing one does not establish the others.

## Start with manifest validation

Run from the repository root after authoring the manifest:

```sh
go test ./connectors/http -run 'Test(Manifests|EmbeddedCatalogRegistration|Catalog)'
go test ./registry
```

These checks cover registry discovery, upstream API version/default selection, aliases, fresh
source instances, and every shipped YAML manifest through grammar validation,
strict decoding, normalization, and semantic checks. Invalid bundled catalog
entries also fail package initialization before tests run. API version checks
cover date/dotted identifiers, absent versions, unsafe paths, and mismatches
between the registry and YAML `api_version`. Fix all reported errors.
They do not test the upstream API, response paths, permissions, or completeness.

## Review the API contract

Do not write provider-specific tests for manifest-based API connectors. Check
requests, response paths, keys, field selectors, pagination termination, parent
selection, and read modes against official documentation and the current engine.
Use existing engine tests to understand supported behavior. Do not treat grammar
validation as proof of upstream fidelity.

## Shared runtime or grammar changes

When connector work requires a shared code change, add focused regression tests
for that behavior. Cover the concrete failure, unchanged defaults, and an
unaffected manifest. For pagination changes, check page transitions and
termination. For decoder or projection changes, check lossless IDs and nested
JSON. For error or throttling rules, check matching and nonmatching responses.
Protect shared state in HTTP fixture handlers.

For shared catalog, version resolution, or connection pinning changes, also run
`go test ./registry ./server ./cmd/internal/cli/...`. Preserve coverage for old
unversioned connections, remote catalog schema lookup, and local document
reapply without silently changing a pinned version.

## Repository checks

Run the existing suite and checks:

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

If live access is unavailable, complete the existing checks and register the new
connector as alpha. List the exact pending checks, including unavailable account
features and inferred response behavior. Do not describe existing tests as
live API verification or silently omit a promised resource.

Completion means the agreed resource scope is implemented, the shared runtime
remains compatible, wiring and docs agree with the behavior, and any remaining
live uncertainty is explicit. Report outcomes rather than every command's output.
