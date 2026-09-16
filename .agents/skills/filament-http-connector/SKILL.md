---
name: filament-http-connector
description: Research a product's official API documentation and build or update a Filament HTTP source connector, including its manifest, catalog registration, docs, and tests. Use for HTTP/SaaS connector work and manifest-versus-driver assessments.
---

# Build an HTTP connector

Deliver a connector that reads the agreed data correctly, fits the existing
HTTP engine, and explains its coverage honestly. Research broadly, then choose
the simplest implementation that meets the user's scope. A large endpoint list
does not establish completeness.

For planning requests, produce the researched design without implementing it.
For a narrow fix, inspect the affected behavior and update only what is needed.
For a new connector, follow the workflow below through wiring and verification.

## 1. Inspect the current implementation

Check the working tree and applicable repository instructions. Read a nearby
manifest, its tests, and its source docs before deciding how to add the connector.
Treat current code as authoritative when a reference or code comment is stale:

- `connectors/http/manifest/grammar.v1.json` defines accepted YAML.
- `connectors/http/manifest/manifest.go`, `validate.go`, and `grammar_enums.go`
  define decoding, defaults, and semantic constraints.
- `connectors/http/source.go`, `paginate.go`, and `pagination/` define runtime
  behavior, connection testing, selection, and recovery.
- `connectors/http/catalog.go` and `register.go` define catalog wiring.

Read [grammar-guide.md](references/grammar-guide.md) when authoring a manifest.
Use [pitfalls.md](references/pitfalls.md) to choose relevant worked examples
and check data correctness.

## 2. Research the product's API before authoring

**Always consult the company's current official API documentation for a new
connector.** Open the user's supplied documentation link. Find the API reference,
authentication guide, pagination and rate-limit rules, relevant endpoint schemas,
and version or deprecation notes. Do not build from product familiarity, search
snippets, another provider's manifest, or an SDK's method names alone.

Follow [research.md](references/research.md). Establish the API's full relevant
read surface, then record a concise coverage matrix with endpoint methods,
response paths, keys, pagination, access requirements, and read modes. Link the
sources behind non-obvious decisions. Separate documented behavior, observed
responses, and assumptions. If documentation is inaccessible, make the missing
evidence explicit and continue independent work without inventing an API contract.

Keep research notes in an existing plan when one exists. Do not create a separate
research document for every small connector. User docs should contain the setup
and behavior users need, not the research transcript.

## 3. Decide whether the manifest fits

Use the current grammar's authentication, request bodies, pagination, projection,
parent/child reads, and discovery before proposing custom code. REST and GraphQL
with JSON responses often fit. A POST query can be a read endpoint.

Supported auth includes static bearer/header credentials, HTTP Basic, and OAuth2
client credentials. The engine does not implement delegated authorization-code
or refresh-token flows. A documented personal or service token can still be a
valid alternative. Do not present a short-lived token as unattended OAuth support.

Separate the decisions:

- **Manifest:** the agreed reads fit existing primitives.
- **Small shared fix:** a concrete correctness gap needs a narrow change that
  preserves existing behavior, such as lossless numeric decoding or an opt-in
  response rule. Explain the need and test the shared path.
- **Driver or revised scope:** essential behavior requires orchestration the
  manifest cannot express, such as runtime property discovery, multi-stage
  hydration, search-limit partitioning, or token lifecycle management.

Do not silently drop a promised core resource to make eligibility pass. Do not
turn a connector task into a new framework, scheduler, or configuration surface.
Honor an explicit manifest-only constraint and identify the precise blocker
when required behavior cannot fit it. If a driver comparison is requested,
inspect the referenced implementation or branch without switching checkouts.

## 4. Design and implement the resource set

Include the core records and supporting reference data needed to interpret them.
Classify meaningful API families as included, optional, or excluded with reasons.
Choose useful defaults. Extra permissions, expensive reads, audit APIs, or separate
product families often belong behind resource selection or outside the initial
scope. Do not exclude useful reference tables merely because they live under
a settings endpoint.

Use existing bulk endpoints before adding per-record requests. Give each resource
an explicit row meaning, a valid key policy, and a trailing `raw` remainder.
Preserve nested data as JSON when splitting it would add speculative tables or
lose relationships. Use `primary_key: []` when no stable key exists and document
the appropriate write mode. Never invent uniqueness to satisfy a convention.

Only declare incremental reads after verifying the upstream filter and watermark
semantics. An event date is not an update timestamp. Full reads are the correct
choice when a date cursor would miss edits or late processing.

Write the manifest, validate it, then follow [wiring.md](references/wiring.md) for
catalog registration, logos, and the Configuration / Resources / Modes / Behavior
docs pattern. Use direct, human prose. Avoid marketing, filler, and semicolon-heavy
sentences.

## 5. Verify and report the actual result

Follow [validation.md](references/validation.md) for mock coverage, shared-runtime
regressions, docs checks, and live smoke tests. Tests must demonstrate requests
and data behavior, not merely repeat the YAML. Fix failures in the changed scope
and distinguish unrelated existing failures.

New connectors start at `filament.MaturityAlpha`. Grammar validation and mocked
responses do not establish live fidelity. Promote maturity only when the user
directs it based on live validation. If credentials or account features are
unavailable, finish the implementation and state exactly which live checks remain.

Finish with resources covered, auth and pagination choices, read modes, checks
run, and material limitations. Avoid promises of perfection or future-proofing.
Leave commits and branch creation to the user unless explicitly requested.

## Multiple products

Apply the same research and validation to each product. When parallel agent work
is authorized and tools are available, give each agent one product's research,
manifest, docs page, and dedicated test file. Keep shared catalog, navigation,
and runtime edits with one coordinator. Review each product's evidence before
final integration. Otherwise work sequentially. Use the tools available in the
active environment.
