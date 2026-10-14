# Catalog wiring and user documentation

Use this after the manifest design is settled. Paths below are relative to the
repository root. Confirm them against the current tree rather than recreating
old pages or constructor APIs.

## Manifest metadata and logos

Write `connectors/http/manifests/<name>/<api-version>/manifest.yaml` for a new connector.
The manifest owns `name`,
`display_name`, `description`, `dark_logo_url`, `light_logo_url`, and
`api_version`, as well as its config schema. Use the connector card description
guidance below for UI metadata.

Prefer the existing catalog CDN convention:

```text
https://cdn.getgalaxy.io/sources/source-icon-<name>-dark.svg
https://cdn.getgalaxy.io/sources/source-icon-<name>-light.svg
```

Verify both URLs actually serve SVGs. A plausible filename is not evidence that
an asset exists. If HEAD is unsupported, check GET. A 403 does not establish
whether an object is absent or private, but it is not a usable public logo.
If a CDN asset is unavailable, use a verified official product asset when
appropriate and report the fallback. Do not invent a working URL or upload
to the CDN as an implied part of connector creation. If no usable asset is
available, identify that remaining dependency.

Use the same dark logo URL in the docs frontmatter. When the user later supplies
CDN assets, update both manifest variants and the docs icon, then check the URLs.

## Connector card descriptions

Apply this convention to the manifest's `description` and driver
`ConnectorSpec.Description` / `SinkSpec.Description`. Describe the external
system in its own official terminology, with enough detail to explain what it
does to someone unfamiliar with the product.

- Start with a concrete product category, such as "CRM platform", "AI notepad",
  or "Column-oriented database". Do not start with "A", "An", or "The", or
  repeat the product name already shown on the card.
- Research the vendor's official product explanation. Prefer an introductory
  documentation or product overview page when the homepage is only a slogan.
  Preserve its category and terminology; adapt grammar and length without
  adding unsupported positioning or capabilities.
- Follow the category with the core purpose or a defining capability: what
  people use it for, how it works, or who it serves. Prefer concrete facts to
  adjectives such as "modern", "powerful", "seamless", or "industry-leading".
- Aim for one sentence of roughly 15-30 words. Keep enough substance to identify
  the product; a generic category or marketing tagline alone is insufficient.
- Describe product capabilities rather than enumerating API resources. Keep
  supported-resource inventories, Filament behavior, checkpoints, ingestion
  modes, and file formats in the resource/configuration docs. A brief product
  or API qualifier is appropriate when it clarifies scope, such as Mailchimp
  Marketing API, FHIR R4, or the S3 API.
- Store plain text only. Keep research citations and documentation links outside
  the description; the UI provides a separate documentation control.
- Use the same description for source and sink entries of the same system unless
  they target different products or interfaces. For utilities such as Sample
  Generator and Standard Output, explain the utility's purpose directly.

Examples:

- Gong: "Revenue intelligence platform that captures and analyzes customer
  interactions to help sales teams understand deals, coach reps, and forecast
  revenue."
- NATS JetStream: "Persistence layer for NATS that stores messages for later
  delivery and replay, allowing publishers and subscribers to communicate at
  different times."

## Register upstream API versions

The HTTP package embeds the entire `manifests` directory and validates its
catalog before registration at startup. Directory names and registry keys are
the upstream API version identifiers. For a provider API named `v3`, use:

```text
connectors/http/manifests/example/
  registry.json
  v3/manifest.yaml
```

Set YAML `api_version: "v3"` and use this registry:

```json
{
  "schema_version": 1,
  "name": "example",
  "default_version": "v3",
  "versions": {
    "v3": { "maturity": "alpha" }
  }
}
```

The registry owns API version selection and per-version maturity. YAML owns API
behavior, configuration, display name, description, and logo URLs. Keep the
registry name, connector directory, and YAML `name` identical; YAML names do not
include `@v3`. Do not add provider-specific embeds, constructors, compatibility
wrappers, or lines in `register.go`. Do not duplicate config schemas in Go.
The binary still needs rebuilding and deploying after embedded files change.

Loader constraints:

- Connector names match `^[a-z][a-z0-9_-]*$`.
- API version keys match `^[A-Za-z0-9][A-Za-z0-9._-]*$`. This permits the
  provider's identifier, such as `v2`, `3.0`, `2022-11-28`, or
  `2026-07-29.dahlia`, while excluding path separators and traversal.
- The version key and directory must exactly match YAML `api_version`.
  When the API has no version, use the reserved `unversioned` key/directory
  and omit YAML `api_version`. Do not invent a vendor `v1`.
- Every listed version has `<version>/manifest.yaml`, every version directory is
  listed, and `default_version` names a listed entry.
- Maturity is `alpha`, `beta`, or `stable` per version; a mature v1 does not make
  a new v2 mature. Keep registry maturity, source docs, and overview consistent.
- Missing files, unknown JSON fields, mismatched names, invalid metadata, and
  invalid manifests fail startup. Keep stray files out of the `manifests` root.

Use the API version already targeted by an existing manifest; confirm it from
its request paths, headers, and the provider's documentation. For a new connector,
research the upstream version rather than assuming the first manifest is `v1`.
For example, the shipped GitHub manifest belongs under `2022-11-28/`, Attio under
`v2/`, and Zoho under `v8/`.

Add another directory only when implementing another upstream API version.
Manifest fixes and new resources targeting the same API stay in that directory.
A breaking Filament-side change is not a new vendor API version: review its
compatibility and migration needs without relabeling it as an upstream release.
Retain API versions used by existing connections. Adding another API version does
not require changing `default_version`; preserve the default unless changing it
is part of the requested work.

Keep these version concepts separate:

| Field or path | Meaning |
|---|---|
| `registry.json` → `schema_version: 1` | Registry JSON format |
| API directory, registry key, catalog `Version` | Upstream API version identifier, or `unversioned` |
| YAML `api_version`, catalog `APIVersion` | Same upstream version; omitted/empty for `unversioned` |
| YAML `version: 1` | Manifest grammar; do not increment for API releases |

## Resolution and existing connections

Resolve bundled sources through the registry with a concrete key such as
`example@v3` or `github@2022-11-28`. The bare `example` alias resolves `default_version`; catalog
listings include both aliases and concrete versions so existing CLI configurations
can look up their schemas. Each resolution constructs a fresh source instance.

The connection creation API stores the concrete version even when given an alias.
Reapplying an unversioned local document preserves an existing concrete API pin. An API pin does not freeze manifest contents; fixes
for the same API still take effect across builds.
Previously stored unversioned connections still follow the default: changing it
can change their behavior. Pin those connections before changing defaults when
preserving their behavior is required; connector authoring itself does not
migrate stored connections or checkpoints. Cross-API-version pipeline upgrades can
require checkpoint migration or reset. During rollout, upgrade workers before
creating connections with versioned keys, which older workers cannot resolve.

## Source page

Create `docs/pages/connectors/sources/<name>.mdx`. Read the nearby source docs,
especially Granola and Gong, and match their prose, formatting, and level of
detail as well as their section structure:

```mdx
---
title: "Example"
description: "Read records and related data from Example"
icon: "https://cdn.getgalaxy.io/sources/source-icon-example-dark.svg"
---
```

Write a short introduction with the supported data, a link to
`/pages/connectors/building-a-connector/http-manifests`, and accurate maturity
and live-validation status. Then use these sections:

| Section | Information users need |
|---|---|
| Configuration | Field / Scope / Default / Description table. Required secrets, credential creation, host or region selection, plans, roles, and access scopes. |
| Resources | Resource / Endpoint / Parent table. Defaults, optional reads, row meaning, important fields, nested JSON, and meaningful exclusions. |
| Modes | Which resources support full or incremental reads, exact cursor semantics and limitations, keyless write-mode guidance, and deletion behavior. |
| Behavior | Pagination, rate limits, parent dependencies, processing or visibility limits, and any special empty/error handling. |

Use `Connection` in the scope column, leave absent defaults and parents blank,
and include the method and full API path in endpoint cells. Write Behavior as
short bullets with bold labels such as **Auth** and **Pagination**. Keep the
introduction brief and put scope exclusions in concise prose. Avoid a research
transcript or a live-validation checklist in the source page.

Scale detail to the connector. A scopes table is useful when resource access
differs, not mandatory for a one-key API. Describe what the user will receive
and what they need to configure. Keep parser mechanics and test implementation
details in the author guide or tests.

Use plain sentences and concrete nouns. Avoid repeated claims of robustness,
completeness, or seamless integration. Prefer short sentences to semicolon chains.
Link official docs near setup instructions and unusual API limitations. Do not
copy large passages or reproduce the full field reference. State known omissions
and unavailable live checks without burying setup under a research diary.

## Documentation cross-references

Update the current locations:

1. `docs/docs.json`: add the source page among the alphabetical SaaS entries.
2. `docs/pages/connectors/overview/introduction.mdx`: add a source row with the
   actual maturity and read modes.

Do not update `docs/pages/connectors/building-a-connector/http-manifests.mdx`
for connector work. Source pages may link to it. Changes to that guide belong
to an explicit documentation task.

The former `sources/http.mdx` card list and `sources/overview.mdx` are not the
current wiring points. Do not restore them or maintain an invented catalog count.

## Validation and completion

Do not add provider-specific tests for manifest-based connectors. Run the
existing checks described in [validation.md](validation.md). Shared runtime or
grammar changes still need focused regression coverage.
