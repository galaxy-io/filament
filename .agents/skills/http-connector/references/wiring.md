# Wiring a new connector

Four mechanical steps after the manifest validates. `<name>` is the manifest's `name:` slug.

## 1. catalog.go — embed + constructor

Add to `connectors/http/catalog.go`, matching the existing entries exactly:

```go
//go:embed manifests/<name>.yaml
var <name>Manifest []byte

// New<Name> returns a Source backed by the embedded <Display> manifest.
func New<Name>() *Source {
	return NewManifestWithMetadata("<name>", "<Display>", "<one-sentence product description>.", "<dark-logo-url>", "<light-logo-url>", <name>Manifest, manifestOwnedConfig())
}
```

Logo URLs follow `https://cdn.getgalaxy.io/sources/source-icon-<name>-dark.svg` / `-light.svg`. If the CDN asset doesn't exist yet, use the pattern anyway and flag it in the final report.

## 2. register.go — one line

```go
registry.RegisterSource("<name>", filament.MaturityAlpha, func() filament.Source { return New<Name>() })
```

Alpha is mandatory by default for a newly authored HTTP connector because its
behavior was derived from API documentation. Manifest validation and
`httptest` coverage verify the implementation mechanically, but do not count as
a live end-to-end run. Only register it as beta or stable when the user
explicitly directs that promotion based on live validation.

## 3. Docs

`docs/pages/connectors/sources/<name>.mdx`, modeled on `github.mdx`:

```mdx
---
title: "<Display>"
description: "<what it reads, one sentence>"
icon: "<dark-logo-url>"
---
```

Body sections: a lead paragraph noting it is manifest-driven (link to `/pages/connectors/sources/http`), `## Auth` (what the user supplies), `## Resources` (table: resource, path, fans out from; then pagination/quirks prose), `## Modes` (full vs incremental). Plain prose, no marketing.

Then three cross-references, all easy to miss:

1. A card in the `CardGroup` in `docs/pages/connectors/sources/http.mdx`, alphabetical:

```mdx
<Card title="<Display>" icon="<dark-logo-url>" href="/pages/connectors/sources/<name>" />
```

2. A nav entry in `docs/docs.json`, alphabetical among the SaaS sources:

```json
"pages/connectors/sources/<name>",
```

3. A row in the SaaS Apps table in `docs/pages/connectors/sources/overview.mdx`
   — **and bump the spelled-out connector count**, which appears three times in
   that file (the "All N run on the same HTTP connector" lead, the Attio row's
   "deepest parent/child nesting of the N", and the Custom & Testing row's
   "the N SaaS sources above run on"). Grep the current number first; it moves
   with every connector added.

## 4. Test in source_test.go

Two patterns, both required:

**Spec + discovery** (model: `TestNewGitHubSpecAndEmbeddedManifest`): construct via `New<Name>()`, assert `Spec().Name`/`DisplayName`, assert each config field's type/required, `Configure` with fake config, `Discover` and assert the exact resource name list.

**Extraction against httptest** (model: the Attio/Slack tests): stub the API, retarget the embedded manifest, extract, assert auth header and records:

```go
manifestData := []byte(strings.Replace(string(<name>Manifest), "<base_url>", api.URL, 1))
src := NewManifest("<name>", "<Display>", manifestData, filament.ConfigSchema{})
```

Cover in the stub: the happy path for at least one resource, one pagination round-trip (first response points at a second page, second terminates), and parent→child fan-out if the manifest uses `for_each`. Use `collectSink` to gather records.

## Verify

```
go test ./connectors/http/...
go vet ./connectors/http/...
go build -o /dev/null ./connectors/http
```

No commits, no branches — the user runs git themselves.
