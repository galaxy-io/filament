package manifest

import (
	"fmt"
	"strconv"
	"strings"
	"time"

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
		if len(r.UseFields) > 0 {
			var inherited FieldList
			for _, name := range r.UseFields {
				inherited = append(inherited, m.FieldSets[name]...)
			}
			r.Fields = append(inherited, r.Fields...)
		}
		if len(r.ExcludeFields) > 0 {
			excluded := make(map[string]struct{}, len(r.ExcludeFields))
			for _, name := range r.ExcludeFields {
				excluded[name] = struct{}{}
			}
			fields := r.Fields[:0]
			for _, field := range r.Fields {
				if _, skip := excluded[field.Name]; !skip {
					fields = append(fields, field)
				}
			}
			r.Fields = fields
		}
		for name, ref := range r.Params {
			r.Path = strings.ReplaceAll(r.Path, "{"+name+"}", referenceTemplate(ref))
		}
		if r.Method == "" {
			r.Method = m.Defaults.Method
		}
		r.Headers = mergeStringDefaults(m.Defaults.Headers, r.Headers)
		r.Query = mergeStringDefaults(m.Defaults.Query, r.Query)
		if r.Response.Root == "" {
			r.Response.Root = m.Defaults.Response.Root
		}
		if r.Response.RecordsPath == "" {
			r.Response.RecordsPath = m.Defaults.Response.RecordsPath
		}
		if r.Response.Cardinality == "" {
			r.Response.Cardinality = m.Defaults.Response.Cardinality
		}
		if r.Response.Error == nil {
			r.Response.Error = m.Defaults.Response.Error
		}
		if r.Response.Records == "" && r.Records == "" {
			r.Response.Records = m.Defaults.Response.Records
		}
		if r.Response.Pagination == nil && r.Pagination.Type == "" {
			r.Response.Pagination = m.Defaults.Response.Pagination
		}
		if r.Response.Pagination != nil {
			r.Pagination = *r.Response.Pagination
		}
		if r.Response.Records != "" {
			r.Records = r.Response.Records
		}
		if r.Pagination.Type == "" {
			r.Pagination = m.Defaults.Pagination
		}
		if r.Records != "" {
			if r.Records == "$" {
				r.Response.Root = "array"
				r.Response.RecordsPath = ""
			} else {
				r.Response.Root = "object"
				r.Response.RecordsPath = strings.TrimPrefix(r.Records, "$.")
			}
		}
		if r.ForEach != "" {
			if r.Parent == nil {
				r.Parent = &ParentRef{}
			}
			if r.Parent.Resource == "" {
				r.Parent.Resource = r.ForEach
			}
		}
		if r.Pagination.Type == "none" {
			r.Pagination.Type = ""
		}
		if r.Mode == "" {
			r.Mode = DefaultMode
		}
		if r.Response.Root == "" {
			r.Response.Root = DefaultResponseRoot
		}
	}
}

func mergeStringDefaults(defaults, local map[string]string) map[string]string {
	if len(defaults) == 0 {
		return local
	}
	out := make(map[string]string, len(defaults)+len(local))
	for k, v := range defaults {
		out[k] = v
	}
	for k, v := range local {
		out[k] = v
	}
	return out
}

// validateSemantics runs every cross-field check and aggregates issues into a
// single error so manifest authors see all problems on one run instead of
// fixing them one at a time. Pairs with ValidateGrammar (shape/enum) — this
// runs after decode, against the typed Manifest, and enforces rules the
// grammar can't express (parent cycles, key collisions, scope-restricted
// templates).
//
//nolint:gocyclo,funlen // linear sequence of independent field validations
func (m *Manifest) validateSemantics() error {
	var agg errs.ManifestErrors

	if m.Name == "" {
		_ = agg.Addf("name", "is required")
	}
	if m.Connection.BaseURL == "" {
		_ = agg.Addf("connection.base_url", "is required")
	}

	// Connection-level templated fields. Headers run per-request (full scope
	// available); base_url and auth params run at connection scope only —
	// `parent` and `cursor` make no sense there and a referenced scope that
	// won't ever resolve must surface at load time, not as a 401 on first
	// request. base_url is rendered once at Configure, so it shares auth's
	// scope set; regional APIs use it to select a host from config.
	for k, v := range m.Connection.Headers {
		validateTemplate(&agg, fmt.Sprintf("connection.headers[%q]", k), v)
	}
	validateTemplateScopes(&agg, "connection.base_url", m.Connection.BaseURL, template.AuthScopes)
	validateAuthParams(&agg, "connection.auth", m.Connection.Auth.Params)

	names := make(map[string]struct{}, len(m.Resources))
	byName := make(map[string]*Resource, len(m.Resources))
	for i := range m.Resources {
		r := &m.Resources[i]
		path := fmt.Sprintf("resources[%d]", i)
		if r.Name == "" {
			_ = agg.Addf(path+".name", "is required")
			continue
		}
		byName[r.Name] = r
		path = fmt.Sprintf("resources[%q]", r.Name)
		for j, fieldSet := range r.UseFields {
			if _, ok := m.FieldSets[fieldSet]; !ok {
				_ = agg.Addf(fmt.Sprintf("%s.use_fields[%d]", path, j), "unknown field set %q", fieldSet)
			}
		}
		if _, dup := names[r.Name]; dup {
			_ = agg.Addf(path, "duplicate resource name")
		}
		names[r.Name] = struct{}{}

		if r.Path == "" {
			_ = agg.Addf(path+".path", "is required")
		}
		if r.Parent != nil && r.Parent.Resource == "" {
			_ = agg.Addf(path+".parent.resource", "is required")
		}
		if r.CaptureOnly {
			if len(r.Capture) == 0 {
				_ = agg.Addf(path+".capture_only", "requires a capture block")
			}
			if r.Incremental != nil {
				_ = agg.Addf(path+".capture_only", "cannot be combined with incremental")
			}
			if r.Mode == "stream" {
				_ = agg.Addf(path+".capture_only", "is not supported for stream resources")
			}
		}

		validateTemplate(&agg, path+".emit_as", r.EmitAs)
		validateTemplate(&agg, path+".path", r.Path)
		fieldNames := make(map[string]struct{}, len(r.Fields))
		for j, f := range r.Fields {
			fieldPath := fmt.Sprintf("%s.fields[%d]", path, j)
			if f.Name == "" {
				_ = agg.Addf(fieldPath+".name", "is required")
			}
			if _, dup := fieldNames[f.Name]; f.Name != "" && dup {
				_ = agg.Addf(fieldPath+".name", "duplicate field name %q", f.Name)
			}
			fieldNames[f.Name] = struct{}{}
			if f.Type == "" {
				_ = agg.Addf(fieldPath+".type", "is required")
			}
			if f.Path == "" && len(f.Shape) == 0 {
				_ = agg.Addf(fieldPath+".path", "either path or shape is required")
			}
			if f.Mode != "" && f.Mode != "raw" && f.Mode != "remainder" {
				_ = agg.Addf(fieldPath+".mode", "must be raw or remainder")
			}
			if f.Mode == "remainder" && f.Type != "json" {
				_ = agg.Addf(fieldPath+".mode", "remainder is only supported for json fields")
			}
			if len(f.Shape) > 0 && f.Type != "json" {
				_ = agg.Addf(fieldPath+".shape", "shape is only supported for json fields")
			}
			if len(f.Shape) > 0 && f.Mode == "remainder" {
				_ = agg.Addf(fieldPath+".shape", "shape cannot be combined with remainder mode")
			}
		}
		for _, pk := range r.PrimaryKey {
			if len(r.Fields) > 0 {
				if _, ok := fieldNames[pk]; !ok {
					_ = agg.Addf(path+".primary_key", "field %q must be declared in fields", pk)
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
			_ = agg.Addf(path+".method", "%v", err)
		}
		if err := checkEnum(r.Mode, ValidModes); err != nil {
			_ = agg.Addf(path+".mode", "%v", err)
		}
		if err := checkEnum(r.Body.Encoding, ValidBodyEncodings); err != nil {
			_ = agg.Addf(path+".body.encoding", "%v", err)
		}
		if err := checkEnum(r.Response.Root, ValidResponseRoots); err != nil {
			_ = agg.Addf(path+".response.root", "%v", err)
		}
		if r.Response.Cardinality != "" && r.Response.Cardinality != "many" && r.Response.Cardinality != "one" {
			_ = agg.Addf(path+".response.cardinality", "must be many or one")
		}
		if err := checkEnum(r.Pagination.Type, ValidPaginationTypes); err != nil {
			_ = agg.Addf(path+".pagination.type", "%v", err)
		}
		if err := checkEnum(r.Pagination.InjectInto, ValidPaginationInject); err != nil {
			_ = agg.Addf(path+".pagination.inject_into", "%v", err)
		}
		if r.Incremental != nil {
			if r.Incremental.CursorField == "" {
				_ = agg.Addf(path+".incremental.cursor_field", "is required")
			}
			if r.Incremental.StartParam == "" {
				_ = agg.Addf(path+".incremental.start_param", "is required")
			}
			if err := checkEnum(r.Incremental.InjectInto, ValidIncrementalInject); err != nil {
				_ = agg.Addf(path+".incremental.inject_into", "%v", err)
			}
			if err := checkEnum(r.Incremental.Comparator, ValidComparators); err != nil {
				_ = agg.Addf(path+".incremental.comparator", "%v", err)
			}
			if r.Incremental.OverlapSeconds > 0 && r.Incremental.Comparator != "time" && r.Incremental.Comparator != "numeric" {
				_ = agg.Addf(path+".incremental.overlap_seconds", "requires comparator time or numeric")
			}
			if err := validateIncrementalInitial(*r.Incremental); err != nil {
				_ = agg.Addf(path+".incremental.initial", "%v", err)
			}
			cursor, found := IncrementalCursorField(*r)
			if !found {
				_ = agg.Addf(path+".incremental.cursor_field", "field %q must be declared and projected in fields", r.Incremental.CursorField)
			} else {
				if cursor.Nullable {
					_ = agg.Addf(path+".incremental.cursor_field", "field %q must be non-nullable", cursor.Name)
				}
				if !incrementalTypeCompatible(r.Incremental.Comparator, cursor.Type) {
					_ = agg.Addf(path+".incremental.comparator", "comparator %q is incompatible with cursor field %q type %q", comparatorName(r.Incremental.Comparator), cursor.Name, cursor.Type)
				}
			}
			if paginationInjectionCollides(r.Pagination, *r.Incremental) {
				_ = agg.Addf(path+".incremental.start_param", "conflicts with the pagination injection target %s.%s", r.Incremental.InjectInto, r.Incremental.StartParam)
			}
		}
		if r.Stream != nil {
			if err := checkEnum(r.Stream.Type, ValidStreamTypes); err != nil {
				_ = agg.Addf(path+".stream.type", "%v", err)
			}
		}
	}

	seenKinds := make(map[string]struct{}, len(m.Discovery.Resources))
	if m.Discovery.Mode == "" {
		m.Discovery.Mode = "static"
	}
	if m.Discovery.Mode != "static" && m.Discovery.Mode != "dynamic" {
		_ = agg.Addf("discovery.mode", "must be static or dynamic")
	}
	if m.Discovery.Mode == "static" && len(m.Discovery.Resources) > 0 {
		_ = agg.Addf("discovery.resources", "must be empty in static mode")
	}
	if m.Discovery.Mode == "dynamic" && len(m.Discovery.Resources) == 0 {
		_ = agg.Addf("discovery.resources", "must not be empty in dynamic mode")
	}
	for i, name := range m.Discovery.Include {
		if _, ok := names[name]; !ok {
			_ = agg.Addf(fmt.Sprintf("discovery.include[%d]", i), "unknown resource %q", name)
		} else if byName[name].CaptureOnly {
			_ = agg.Addf(fmt.Sprintf("discovery.include[%d]", i), "resource %q is capture_only", name)
		}
	}
	if m.Discovery.Mode == "dynamic" && len(m.Discovery.Include) > 0 {
		_ = agg.Addf("discovery.include", "is only supported in static mode")
	}
	for i := range m.Discovery.Resources {
		path := fmt.Sprintf("discovery[%d]", i)
		validateDiscovery(&agg, path, &m.Discovery.Resources[i], names)
		kind := m.Discovery.Resources[i].Map.Kind
		if kind == "" {
			continue
		}
		if _, dup := seenKinds[kind]; dup {
			_ = agg.Addf(path+".map.kind", "duplicate kind %q across discovery entries", kind)
		}
		seenKinds[kind] = struct{}{}
	}

	if m.Connection.RateLimit.Dynamic != nil {
		if err := checkEnum(m.Connection.RateLimit.Dynamic.ResetFormat, ValidRateLimitResetFormats); err != nil {
			_ = agg.Addf("connection.rate_limit.dynamic.reset_format", "%v", err)
		}
	}

	// Parent reference resolution.
	for _, r := range m.Resources {
		if r.Parent == nil {
			continue
		}
		parent, ok := byName[r.Parent.Resource]
		if !ok {
			_ = agg.Addf(fmt.Sprintf("resources[%q].parent.resource", r.Name),
				"unknown parent %q", r.Parent.Resource)
			continue
		}
		if r.Parent.Since == "" {
			continue
		}
		if r.Incremental == nil {
			_ = agg.Addf(fmt.Sprintf("resources[%q].parent.since", r.Name), "requires an incremental block")
		}
		if _, captured := parent.Capture[r.Parent.Since]; !captured {
			_ = agg.Addf(fmt.Sprintf("resources[%q].parent.since", r.Name),
				"field %q is not captured by parent %q", r.Parent.Since, r.Parent.Resource)
		}
	}

	// Cycle detection on parent refs. A two-cycle (A → B → A) would crash
	// SortResources with a stack overflow; a longer cycle would loop
	// forever in extract. Catch at load.
	if cycle := detectParentCycle(m.Resources); cycle != nil {
		_ = agg.Addf("resources",
			"parent reference cycle: %s", formatCycle(cycle))
	}

	// CheckpointKey collisions silently destroy each other's persisted
	// cursor. Reject at load with both owners named.
	seen := map[string]string{}
	for _, r := range m.Resources {
		if r.Incremental == nil {
			continue
		}
		key := r.Incremental.DurableCheckpointKey()
		if owner, dup := seen[key]; dup {
			_ = agg.Addf(fmt.Sprintf("resources[%q].incremental.checkpoint_key", r.Name),
				"key %q collides with resources[%q]", key, owner)
		}
		seen[key] = r.Name
	}

	return agg.AsError()
}

// IncrementalCursorField resolves the declared, projectable cursor field for a
// resource. Validation and extraction share this resolver so they cannot
// disagree about parent-scoped or missing fields.
func IncrementalCursorField(resource Resource) (FieldSpec, bool) {
	if resource.Incremental == nil {
		return FieldSpec{}, false
	}
	for _, field := range resource.Fields {
		if field.Name == resource.Incremental.CursorField {
			if strings.HasPrefix(field.Path, "parent.") {
				return FieldSpec{}, false
			}
			return field, true
		}
	}
	return FieldSpec{}, false
}

func comparatorName(name string) string {
	if name == "" {
		return "lex"
	}
	return name
}

func incrementalTypeCompatible(comparator, fieldType string) bool {
	typ := strings.ToLower(strings.TrimSpace(fieldType))
	switch comparatorName(comparator) {
	case "numeric":
		switch typ {
		case "string", "int", "int16", "int32", "int64", "float", "float32", "float64", "decimal", "number":
			return true
		}
	case "time":
		return typ == "string" || typ == "timestamp" || typ == "timestamptz"
	case "lex":
		switch typ {
		case "string", "uuid", "date", "time", "timestamp", "timestamptz":
			return true
		}
	}
	return false
}

func validateIncrementalInitial(spec IncrementalSpec) error {
	if spec.Initial == "" {
		return nil
	}
	switch comparatorName(spec.Comparator) {
	case "numeric":
		if _, err := strconv.ParseFloat(spec.Initial, 64); err != nil {
			return fmt.Errorf("must be numeric")
		}
	case "time":
		if _, err := time.Parse(time.RFC3339, spec.Initial); err != nil {
			return fmt.Errorf("must be RFC3339")
		}
	}
	return nil
}

func paginationInjectionCollides(pagination PaginationSpec, incremental IncrementalSpec) bool {
	target, param := "", ""
	switch pagination.Type {
	case "cursor":
		target, param = pagination.InjectInto, pagination.CursorParam
	case "offset":
		if pagination.OffsetParam == incremental.StartParam {
			target, param = pagination.OffsetInjectInto, pagination.OffsetParam
		} else {
			target, param = pagination.OffsetInjectInto, pagination.LimitParam
		}
	case "page":
		target = "query"
		if pagination.PageParam == incremental.StartParam {
			param = pagination.PageParam
		} else {
			param = pagination.SizeParam
		}
	}
	return target == incremental.InjectInto && param == incremental.StartParam
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
		_ = agg.Addf(path, "%v", err)
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
		_ = agg.Addf(path+".from", "is required")
	} else if _, ok := names[d.From]; !ok {
		_ = agg.Addf(path+".from", "unknown resource %q", d.From)
	}
	if d.Map.Kind == "" {
		_ = agg.Addf(path+".map.kind", "is required")
	}
	if d.Map.IDPath == "" {
		_ = agg.Addf(path+".map.id", "is required")
	}
	if d.Map.NamePath == "" && len(d.Map.NamePaths) == 0 {
		_ = agg.Addf(path+".map.name", "either map.name or map.name_paths is required")
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
		_ = agg.Addf(path+".scope.field", "is required when scope is set")
	}
	if d.Scope.InjectInto == "" {
		_ = agg.Addf(path+".scope.inject_into", "is required when scope is set")
	} else if err := checkEnum(d.Scope.InjectInto, ValidPaginationInject); err != nil {
		_ = agg.Addf(path+".scope.inject_into", "%v", err)
	}
	if len(d.Scope.AppliesTo) == 0 {
		_ = agg.Addf(path+".scope.applies_to", "must list at least one resource when scope is set")
	}
	for i, target := range d.Scope.AppliesTo {
		if _, ok := names[target]; !ok {
			_ = agg.Addf(fmt.Sprintf("%s.scope.applies_to[%d]", path, i),
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
