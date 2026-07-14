package httpapi

import (
	"context"
	"encoding/json"
	"fmt"

	"github.com/galaxy-io/filament/connectors/http/internal/paths"
	"github.com/galaxy-io/filament/connectors/http/internal/pipeline"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/connectors/http/obs"
	"github.com/galaxy-io/filament/connectors/http/pagination"
	"github.com/galaxy-io/filament/connectors/http/response"
)

// Discover enumerates togglable resources by re-running the manifest-declared
// Discovery.From resource through the standard request+pagination machinery,
// then projecting each record via Discovery.Map. Connectors without a
// `discovery:` block return an empty result — callers should fall back to
// treating the connector as a single implicit resource.
//
// Reuses fetchPage so auth, rate limiting, retries, and pagination semantics
// stay identical to extraction.
func (c *Connector) Discover(ctx context.Context, opts pipeline.DiscoverOptions) (*pipeline.DiscoverResult, error) {
	if c.manifest == nil {
		return nil, fmt.Errorf("connector not configured")
	}
	if len(c.manifest.Discovery) == 0 {
		return &pipeline.DiscoverResult{}, nil
	}

	c.logger = obs.Logger(opts.Logger)
	c.reporter = obs.Reporter(c.reporter)

	kindFilter := make(map[string]struct{}, len(opts.Kinds))
	for _, k := range opts.Kinds {
		kindFilter[k] = struct{}{}
	}

	var out []pipeline.Resource
	for i := range c.manifest.Discovery {
		disc := &c.manifest.Discovery[i]
		if len(kindFilter) > 0 {
			if _, ok := kindFilter[disc.Map.Kind]; !ok {
				continue
			}
		}
		resources, err := c.discoverOne(ctx, disc)
		if err != nil {
			return nil, err
		}
		out = append(out, resources...)
	}
	return &pipeline.DiscoverResult{Resources: out}, nil
}

func (c *Connector) discoverOne(ctx context.Context, disc *manifest.Discovery) ([]pipeline.Resource, error) {
	res, ok := findResource(c.manifest.Resources, disc.From)
	if !ok {
		return nil, fmt.Errorf("discovery.from references unknown resource %q", disc.From)
	}

	pag, err := pagination.New(res.Pagination)
	if err != nil {
		return nil, fmt.Errorf("discovery paginator: %w", err)
	}
	extractor := response.New(res.Response)

	var state pagination.State
	if pag != nil {
		state = pag.Initial()
	}

	var out []pipeline.Resource
	for {
		resp, raw, err := c.fetchPage(ctx, res, nil, state, pag, nil)
		if err != nil {
			return nil, fmt.Errorf("discovery fetch %s: %w", disc.From, err)
		}
		if err := extractor.CheckError(raw); err != nil {
			return nil, fmt.Errorf("discovery response %s: %w", disc.From, err)
		}
		records, err := extractor.Records(raw)
		if err != nil {
			return nil, fmt.Errorf("discovery decode %s: %w", disc.From, err)
		}
		for _, rec := range records {
			r, ok := projectResource(rec, disc.Map)
			if !ok {
				continue
			}
			out = append(out, r)
		}

		if pag == nil {
			break
		}
		var bodyMap map[string]any
		_ = json.Unmarshal(raw, &bodyMap)
		state, err = pag.Next(resp, bodyMap, len(records))
		if err != nil {
			return nil, fmt.Errorf("discovery paginate %s: %w", disc.From, err)
		}
		if state.Done {
			break
		}
	}
	return out, nil
}

// projectResource turns one raw record from the Discovery.From stream into a
// pipeline.Resource via the manifest's ResourceMap. Records missing the id
// path are skipped (returned ok=false) rather than failing the whole pass —
// an upstream API quirk should not abort discovery.
func projectResource(rec map[string]any, m manifest.ResourceMap) (pipeline.Resource, bool) {
	id, _, err := paths.AsString(rec, m.IDPath)
	if err != nil || id == "" {
		return pipeline.Resource{}, false
	}
	name := resolveName(rec, m)

	var parent string
	if m.ParentIDPath != "" {
		parent, _, _ = paths.AsString(rec, m.ParentIDPath)
	}

	defaultEnabled := true
	if m.DefaultEnabled != "" {
		// v1: treat DefaultEnabled as a JSON path resolving to bool.
		// Templates (e.g. "{{ not .is_private }}") are not evaluated yet —
		// follow-up will add template support if needed.
		if v, err := paths.Bool(rec, m.DefaultEnabled); err == nil {
			defaultEnabled = v
		}
	}

	meta := make(map[string]string, len(m.Metadata))
	for k, path := range m.Metadata {
		if s, _, err := paths.AsString(rec, path); err == nil {
			meta[k] = s
		}
	}

	var group string
	if m.GroupPath != "" {
		group, _, _ = paths.AsString(rec, m.GroupPath)
	}

	return pipeline.Resource{
		Kind:           m.Kind,
		ID:             id,
		Name:           name,
		ParentID:       parent,
		Group:          group,
		DefaultEnabled: defaultEnabled,
		Metadata:       meta,
	}, true
}

// resolveName walks NamePaths in order, then NamePath, returning the first
// non-empty resolution. For Notion pages where the title property name varies
// per database, this lets a manifest list every common candidate and the
// extractor picks whichever exists on the record.
func resolveName(rec map[string]any, m manifest.ResourceMap) string {
	candidates := m.NamePaths
	if len(candidates) == 0 && m.NamePath != "" {
		candidates = []string{m.NamePath}
	}
	for _, p := range candidates {
		if s, _, err := paths.AsString(rec, p); err == nil && s != "" {
			return s
		}
	}
	// Last-resort fallback: scan properties.* for any value whose `type`
	// is "title" and pull its first plain_text. Works for Notion pages
	// regardless of which property holds the title (database-defined
	// title columns can be named anything).
	if props, ok := rec["properties"].(map[string]any); ok {
		for _, v := range props {
			prop, ok := v.(map[string]any)
			if !ok {
				continue
			}
			if t, _ := prop["type"].(string); t != "title" {
				continue
			}
			arr, ok := prop["title"].([]any)
			if !ok || len(arr) == 0 {
				continue
			}
			first, ok := arr[0].(map[string]any)
			if !ok {
				continue
			}
			if s, ok := first["plain_text"].(string); ok && s != "" {
				return s
			}
		}
	}
	return ""
}

func findResource(resources []manifest.Resource, name string) (manifest.Resource, bool) {
	for _, r := range resources {
		if r.Name == name {
			return r, true
		}
	}
	return manifest.Resource{}, false
}
