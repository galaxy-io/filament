package manifest

import (
	"fmt"

	"github.com/galaxy-io/filament/connectors/http/errs"
	"github.com/galaxy-io/filament/connectors/http/template"
)

// Default values applied by Normalize. Documented here rather than scattered
// across schema comments so the source of truth is the code.
const (
	DefaultMode         = "paginated"
	DefaultResponseRoot = "object"
)

// Normalize fills in declared defaults. Run before validate so subsequent
// checks see resolved values rather than empty fields.
func (m *Manifest) Normalize() {
	for i := range m.Resources {
		r := &m.Resources[i]
		if r.Mode == "" {
			r.Mode = DefaultMode
		}
		if r.Response.Root == "" {
			r.Response.Root = DefaultResponseRoot
		}
	}
}

// validateSemantics runs every cross-field check and aggregates issues into a
// single error so manifest authors see all problems on one run instead of
// fixing them one at a time. Pairs with ValidateGrammar (shape/enum) — this
// runs after decode, against the typed Manifest, and enforces rules the
// grammar can't express (parent cycles, key collisions, scope-restricted
// templates).
func (m *Manifest) validateSemantics() error {
	var agg errs.ManifestErrors

	if m.Name == "" {
		agg.Addf("name", "is required")
	}
	if m.Connection.BaseURL == "" {
		agg.Addf("connection.base_url", "is required")
	}

	// Connection-level templated fields. Headers run per-request (full scope
	// available); auth params run at connection scope only — `parent` and
	// `cursor` make no sense there and a referenced scope that won't ever
	// resolve must surface at load time, not as a 401 on first request.
	for k, v := range m.Connection.Headers {
		validateTemplate(&agg, fmt.Sprintf("connection.headers[%q]", k), v)
	}
	validateAuthParams(&agg, "connection.auth", m.Connection.Auth.Params)

	names := make(map[string]struct{}, len(m.Resources))
	for i := range m.Resources {
		r := &m.Resources[i]
		path := fmt.Sprintf("resources[%d]", i)
		if r.Name == "" {
			agg.Addf(path+".name", "is required")
			continue
		}
		path = fmt.Sprintf("resources[%q]", r.Name)
		if _, dup := names[r.Name]; dup {
			agg.Addf(path, "duplicate resource name")
		}
		names[r.Name] = struct{}{}

		if r.Path == "" {
			agg.Addf(path+".path", "is required")
		}
		if r.Parent != nil && r.Parent.Resource == "" {
			agg.Addf(path+".parent.resource", "is required")
		}

		validateTemplate(&agg, path+".emit_as", r.EmitAs)
		validateTemplate(&agg, path+".path", r.Path)
		fieldNames := make(map[string]struct{}, len(r.Fields))
		for j, f := range r.Fields {
			fieldPath := fmt.Sprintf("%s.fields[%d]", path, j)
			if f.Name == "" {
				agg.Addf(fieldPath+".name", "is required")
			}
			if _, dup := fieldNames[f.Name]; f.Name != "" && dup {
				agg.Addf(fieldPath+".name", "duplicate field name %q", f.Name)
			}
			fieldNames[f.Name] = struct{}{}
			if f.Type == "" {
				agg.Addf(fieldPath+".type", "is required")
			}
			if f.Path == "" && len(f.Shape) == 0 {
				agg.Addf(fieldPath+".path", "either path or shape is required")
			}
			if f.Mode != "" && f.Mode != "raw" && f.Mode != "remainder" {
				agg.Addf(fieldPath+".mode", "must be raw or remainder")
			}
			if f.Mode == "remainder" && f.Type != "json" {
				agg.Addf(fieldPath+".mode", "remainder is only supported for json fields")
			}
			if len(f.Shape) > 0 && f.Type != "json" {
				agg.Addf(fieldPath+".shape", "shape is only supported for json fields")
			}
			if len(f.Shape) > 0 && f.Mode == "remainder" {
				agg.Addf(fieldPath+".shape", "shape cannot be combined with remainder mode")
			}
		}
		for _, pk := range r.PrimaryKey {
			if len(r.Fields) > 0 {
				if _, ok := fieldNames[pk]; !ok {
					agg.Addf(path+".primary_key", "field %q must be declared in fields", pk)
				}
			}
		}
		for k, v := range r.Query {
			validateTemplate(&agg, fmt.Sprintf("%s.query[%q]", path, k), v)
		}
		for k, v := range r.Headers {
			validateTemplate(&agg, fmt.Sprintf("%s.headers[%q]", path, k), v)
		}
		validateTemplateAny(&agg, path+".body.template", r.Body.Template)

		// Enum membership. KnownFields catches unknown field names; these
		// catch unknown values for known fields. Empty string is permitted
		// for fields that have a Normalize-applied default; Normalize ran
		// before validateSemantics so any "" still here means the manifest
		// authors intentionally cleared it.
		if err := checkEnum(r.Method, ValidHTTPMethods); err != nil {
			agg.Addf(path+".method", "%v", err)
		}
		if err := checkEnum(r.Mode, ValidModes); err != nil {
			agg.Addf(path+".mode", "%v", err)
		}
		if err := checkEnum(r.Body.Encoding, ValidBodyEncodings); err != nil {
			agg.Addf(path+".body.encoding", "%v", err)
		}
		if err := checkEnum(r.Response.Root, ValidResponseRoots); err != nil {
			agg.Addf(path+".response.root", "%v", err)
		}
		if err := checkEnum(r.Pagination.Type, ValidPaginationTypes); err != nil {
			agg.Addf(path+".pagination.type", "%v", err)
		}
		if err := checkEnum(r.Pagination.InjectInto, ValidPaginationInject); err != nil {
			agg.Addf(path+".pagination.inject_into", "%v", err)
		}
		if r.Incremental != nil {
			if err := checkEnum(r.Incremental.InjectInto, ValidIncrementalInject); err != nil {
				agg.Addf(path+".incremental.inject_into", "%v", err)
			}
			if err := checkEnum(r.Incremental.Comparator, ValidComparators); err != nil {
				agg.Addf(path+".incremental.comparator", "%v", err)
			}
		}
		if r.Stream != nil {
			if err := checkEnum(r.Stream.Type, ValidStreamTypes); err != nil {
				agg.Addf(path+".stream.type", "%v", err)
			}
		}
	}

	seenKinds := make(map[string]struct{}, len(m.Discovery))
	for i := range m.Discovery {
		path := fmt.Sprintf("discovery[%d]", i)
		validateDiscovery(&agg, path, &m.Discovery[i], names)
		kind := m.Discovery[i].Map.Kind
		if kind == "" {
			continue
		}
		if _, dup := seenKinds[kind]; dup {
			agg.Addf(path+".map.kind", "duplicate kind %q across discovery entries", kind)
		}
		seenKinds[kind] = struct{}{}
	}

	if m.Connection.RateLimit.Dynamic != nil {
		if err := checkEnum(m.Connection.RateLimit.Dynamic.ResetFormat, ValidRateLimitResetFormats); err != nil {
			agg.Addf("connection.rate_limit.dynamic.reset_format", "%v", err)
		}
	}

	// Parent reference resolution.
	for _, r := range m.Resources {
		if r.Parent == nil {
			continue
		}
		if _, ok := names[r.Parent.Resource]; !ok {
			agg.Addf(fmt.Sprintf("resources[%q].parent.resource", r.Name),
				"unknown parent %q", r.Parent.Resource)
		}
	}

	// Cycle detection on parent refs. A two-cycle (A → B → A) would crash
	// SortResources with a stack overflow; a longer cycle would loop
	// forever in extract. Catch at load.
	if cycle := detectParentCycle(m.Resources); cycle != nil {
		agg.Addf("resources",
			"parent reference cycle: %s", formatCycle(cycle))
	}

	// CheckpointKey collisions silently destroy each other's persisted
	// cursor. Reject at load with both owners named.
	seen := map[string]string{}
	for _, r := range m.Resources {
		if r.Incremental == nil {
			continue
		}
		key := r.Incremental.CheckpointKey
		if key == "" {
			key = r.Incremental.CursorField
		}
		if owner, dup := seen[key]; dup {
			agg.Addf(fmt.Sprintf("resources[%q].incremental.checkpoint_key", r.Name),
				"key %q collides with resources[%q]", key, owner)
		}
		seen[key] = r.Name
	}

	return agg.AsError()
}

// validateTemplate parses s and records any syntax/semantic issue against
// the supplied path. Pure-text strings (no `{{`) are skipped so simple
// values aren't paid for.
func validateTemplate(agg *errs.ManifestErrors, path, s string) {
	validateTemplateScopes(agg, path, s, template.AllowedScopes)
}

// validateTemplateScopes is validateTemplate with caller-restricted scopes.
// Use the narrowest set legal at the position so a template that names a
// scope unavailable in its context fails at load.
func validateTemplateScopes(agg *errs.ManifestErrors, path, s string, allowed []string) {
	if s == "" {
		return
	}
	if err := template.Validate(s, allowed); err != nil {
		agg.Addf(path, "%v", err)
	}
}

// validateAuthParams walks every templated string leaf in auth params and
// validates against the auth scope subset. Mirrors auth.validateLeaf's
// recursion so chain step bodies are checked too.
func validateAuthParams(agg *errs.ManifestErrors, path string, v any) {
	switch x := v.(type) {
	case string:
		validateTemplateScopes(agg, path, x, template.AuthScopes)
	case map[string]any:
		for k, val := range x {
			validateAuthParams(agg, path+"."+k, val)
		}
	case []any:
		for i, val := range x {
			validateAuthParams(agg, fmt.Sprintf("%s[%d]", path, i), val)
		}
	}
}

// validateTemplateAny recursively descends body.Template (which is `any`)
// and validates each string leaf. Non-string scalars pass through.
func validateTemplateAny(agg *errs.ManifestErrors, path string, v any) {
	switch x := v.(type) {
	case string:
		validateTemplate(agg, path, x)
	case map[string]any:
		for k, val := range x {
			validateTemplateAny(agg, fmt.Sprintf("%s.%s", path, k), val)
		}
	case []any:
		for i, val := range x {
			validateTemplateAny(agg, fmt.Sprintf("%s[%d]", path, i), val)
		}
	case map[string]string:
		for k, val := range x {
			validateTemplate(agg, fmt.Sprintf("%s.%s", path, k), val)
		}
	}
}

// detectParentCycle returns the cycle path (slice of resource names ending
// with the same name it started with) when one exists, else nil. Uses
// iterative-coloring DFS — white, gray (in-progress), black (done) — so a
// gray-to-gray edge identifies the cycle.
func detectParentCycle(resources []Resource) []string {
	const (
		white = 0
		gray  = 1
		black = 2
	)
	color := make(map[string]int, len(resources))
	parent := make(map[string]string, len(resources))
	for _, r := range resources {
		color[r.Name] = white
		if r.Parent != nil {
			parent[r.Name] = r.Parent.Resource
		}
	}

	var cycle []string
	var dfs func(name string, path []string) bool
	dfs = func(name string, path []string) bool {
		switch color[name] {
		case gray:
			for i, p := range path {
				if p == name {
					cycle = append(append([]string{}, path[i:]...), name)
					return true
				}
			}
			return true
		case black:
			return false
		}
		color[name] = gray
		path = append(path, name)
		if p, ok := parent[name]; ok {
			if _, exists := color[p]; exists {
				if dfs(p, path) {
					return true
				}
			}
		}
		color[name] = black
		return false
	}

	for _, r := range resources {
		if color[r.Name] == white {
			if dfs(r.Name, nil) {
				return cycle
			}
		}
	}
	return nil
}

// validateDiscovery enforces shape rules the grammar cannot express:
// Discovery.From must reference an existing resource, required projection
// paths must be non-empty, and the scope spec must name an injection target
// and at least one applies_to resource (otherwise the scope is dead code).
func validateDiscovery(agg *errs.ManifestErrors, path string, d *Discovery, names map[string]struct{}) {
	if d.From == "" {
		agg.Addf(path+".from", "is required")
	} else if _, ok := names[d.From]; !ok {
		agg.Addf(path+".from", "unknown resource %q", d.From)
	}
	if d.Map.Kind == "" {
		agg.Addf(path+".map.kind", "is required")
	}
	if d.Map.IDPath == "" {
		agg.Addf(path+".map.id", "is required")
	}
	if d.Map.NamePath == "" && len(d.Map.NamePaths) == 0 {
		agg.Addf(path+".map.name", "either map.name or map.name_paths is required")
	}
	if d.Map.DefaultEnabled != "" {
		validateTemplate(agg, path+".map.default_enabled", d.Map.DefaultEnabled)
	}

	// Scope is OPTIONAL. Some discovery entries (e.g. notion pages discovered
	// to filter the parent-capture set) don't need per-request injection
	// because filtering happens against captured parent ids before child
	// fan-out. If any scope subfield is set, all of the required ones must be.
	if d.Scope.Field == "" && d.Scope.InjectInto == "" && len(d.Scope.AppliesTo) == 0 {
		return
	}
	if d.Scope.Field == "" {
		agg.Addf(path+".scope.field", "is required when scope is set")
	}
	if d.Scope.InjectInto == "" {
		agg.Addf(path+".scope.inject_into", "is required when scope is set")
	} else if err := checkEnum(d.Scope.InjectInto, ValidPaginationInject); err != nil {
		agg.Addf(path+".scope.inject_into", "%v", err)
	}
	if len(d.Scope.AppliesTo) == 0 {
		agg.Addf(path+".scope.applies_to", "must list at least one resource when scope is set")
	}
	for i, target := range d.Scope.AppliesTo {
		if _, ok := names[target]; !ok {
			agg.Addf(fmt.Sprintf("%s.scope.applies_to[%d]", path, i),
				"unknown resource %q", target)
		}
	}
}

func formatCycle(cycle []string) string {
	if len(cycle) == 0 {
		return ""
	}
	out := cycle[0]
	for _, n := range cycle[1:] {
		out += " → " + n
	}
	return out
}
