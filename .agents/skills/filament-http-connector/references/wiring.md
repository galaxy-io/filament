# Catalog wiring and user documentation

Use this after the manifest design is settled. Paths below are relative to the
repository root. Confirm them against the current tree rather than recreating
old pages or constructor APIs.

## Manifest metadata and logos

Write `connectors/http/manifests/<name>.yaml`. The manifest owns `name`,
`display_name`, `description`, `dark_logo_url`, `light_logo_url`, and
`api_version`, as well as its config schema. Describe the data the connector reads, not the product's
marketing positioning.

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

## Embed and register

In `connectors/http/catalog.go`, place the embed and manifest variable with the
other embed declarations at the top, and the constructor with the other
constructors. Do not append the embed after the functions:

```go
//go:embed manifests/example.yaml
var exampleManifest []byte

// NewExample returns a Source backed by the embedded Example manifest.
func NewExample() *Source {
    return newCatalogSource(exampleManifest)
}
```

Add the registration in `connectors/http/register.go`:

```go
registry.RegisterSource("example", filament.MaturityAlpha, func() filament.Source { return NewExample() })
```

Match current naming and formatting. `newCatalogSource` calls `NewManifest(data)`.
Do not use removed metadata constructors or duplicate the config schema in Go.
Check registry callers if the catalog structure changes. Keep maturity consistent
between registration, docs, and the source overview.

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
