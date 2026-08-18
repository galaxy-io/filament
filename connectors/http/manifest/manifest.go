// Package manifest defines the v1 YAML connector manifest schema and loader
// for the generic HTTP connector. A manifest describes the connection,
// authentication, rate limits, and a set of resources (endpoints) to extract.
//
// A manifest MUST set `version: 1`. Any other value is rejected.
//
// # Worked example
//
// Minimal Notion-style cursor-paginated manifest:
//
//	version: 1
//	name: notion
//	connection:
//	  base_url: https://api.notion.com/v1
//	  auth: { type: bearer, token: "{{ config.api_key }}" }
//	resources:
//	  - name: databases
//	    path: /databases
//	    method: GET
//	    primary_key: [id]
//	    response: { root: object, records_path: results }
//	    pagination:
//	      type: cursor
//	      cursor_path: next_cursor
//	      cursor_param: start_cursor
//	      has_more_path: has_more
//	      inject_into: query
//
//	m, err := manifest.Load("manifest.yaml")
//
// # Validation
//
// Load runs schema check, YAML decode, version gate, Normalize (defaults),
// then validateSemantics (every issue surfaced in one error). Aggregated errors
// are wrapped in errs.ErrManifestValidate so callers can dispatch with
// errors.Is and inspect issues via errors.As to a *errs.ManifestErrors.
//
// # Failure modes
//
//   - File missing / unreadable     → "read manifest: ..."
//   - JSON-schema mismatch          → "manifest schema: ..."
//   - YAML parse error              → "parse manifest: ..."
//   - Wrong version                 → "unsupported manifest version N (want 1)"
//   - Required-field/template/cycle → wrapped errs.ErrManifestValidate with
//     all issues listed (see validateSemantics)
package manifest

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	"gopkg.in/yaml.v3"
)

// SupportedVersion is the manifest schema version this package accepts.
const SupportedVersion = 1

// Manifest is the root document describing one HTTP API connector.
type Manifest struct {
	Version    int                   `yaml:"version"`
	Name       string                `yaml:"name"`
	Config     map[string]ConfigSpec `yaml:"config,omitempty"`
	Defaults   Defaults              `yaml:"defaults,omitempty"`
	FieldSets  map[string]FieldList  `yaml:"field_sets,omitempty"`
	Connection Connection            `yaml:"connection"`
	Resources  []Resource            `yaml:"resources"`
	// Discovery, when set, lets the connector enumerate user-toggleable
	// resources by reusing existing extraction streams. Each entry projects a
	// distinct resource Kind (e.g. notion exposes both "database" and "page"
	// as togglable). Enabled selections are then injected back into other
	// streams via each entry's Scope.
	Discovery DiscoverySpec `yaml:"discovery,omitempty"`
}

// DiscoverySpec chooses stable manifest resources or dynamically projected
// upstream objects. Static discovery performs no HTTP requests.
type DiscoverySpec struct {
	Mode      string      `yaml:"mode"` // static | dynamic
	Include   []string    `yaml:"include,omitempty"`
	Resources []Discovery `yaml:"resources,omitempty"`
}

// ConfigSpec declares one user-facing connector configuration field. Enum is
// the ordered set of choices for enum and list fields.
type ConfigSpec struct {
	Type     string   `yaml:"type"`
	Required bool     `yaml:"required,omitempty"`
	Default  any      `yaml:"default,omitempty"`
	Enum     []string `yaml:"enum,omitempty"`
	Help     string   `yaml:"help,omitempty"`
	Scope    string   `yaml:"scope,omitempty"`
}

// Defaults are inherited by resources when the corresponding resource field
// is omitted. Resource-local values always win.
type Defaults struct {
	Method     string            `yaml:"method,omitempty"`
	Headers    map[string]string `yaml:"headers,omitempty"`
	Query      map[string]string `yaml:"query,omitempty"`
	Response   ResponseSpec      `yaml:"response,omitempty"`
	Pagination PaginationSpec    `yaml:"pagination,omitempty"`
}

// Discovery declares how to enumerate togglable resources from a manifest. It
// reuses an existing Resource (named by From) as the discovery query, so no
// new HTTP plumbing is required.
type Discovery struct {
	From  string      `yaml:"from"`  // name of the Resource that lists togglable items
	Map   ResourceMap `yaml:"map"`   // projection from a record to a discoverable resource
	Scope ScopeSpec   `yaml:"scope"` // how enabled selections filter extraction
}

// ResourceMap projects a JSON record (from the Discovery.From stream) to
// discoverable-resource fields. Paths are JSONPath-style ($.foo.bar).
type ResourceMap struct {
	Kind         string `yaml:"kind"` // literal value, e.g. "channel"
	IDPath       string `yaml:"id"`   // required
	NamePath     string `yaml:"name"` // single path OR alias; see NamePaths
	ParentIDPath string `yaml:"parent_id,omitempty"`
	// NamePaths is an ordered list of fallback paths; the first non-empty
	// resolution wins. Use it for shapes like Notion pages where the title
	// lives under a property whose key varies per database. When set, takes
	// precedence over NamePath; either field individually is sufficient.
	NamePaths      []string          `yaml:"name_paths,omitempty"`
	DefaultEnabled string            `yaml:"default_enabled,omitempty"`
	Metadata       map[string]string `yaml:"metadata,omitempty"`
	// GroupPath resolves to a string used as the picker's group bucket
	// label (e.g. the parent database title for a Notion page). Empty =
	// ungrouped.
	GroupPath string `yaml:"group_path,omitempty"`
}

// ScopeSpec describes how the enabled resource set filters extraction of OTHER
// streams. For fan-out scopes (e.g. one request per enabled channel id) the
// runtime issues N requests substituting {{ .scope.<Field> }} per id.
type ScopeSpec struct {
	Field      string   `yaml:"field"`       // e.g. "channel_id"
	InjectInto string   `yaml:"inject_into"` // query | path | body | header
	AppliesTo  []string `yaml:"applies_to"`  // resource names that require this scope
	FanOut     bool     `yaml:"fan_out"`     // true = one request per enabled id
}

// Connection configures the base URL, auth, rate limit, and shared headers.
type Connection struct {
	BaseURL        string            `yaml:"base_url"`
	Auth           AuthSpec          `yaml:"auth"`
	RateLimit      RateLimit         `yaml:"rate_limit"`
	Headers        map[string]string `yaml:"headers"`
	TimeoutSeconds int               `yaml:"timeout_seconds"`
}

// AuthSpec is a loose, strategy-keyed auth descriptor. The `type` field selects
// an authenticator implementation (bearer|header|query|basic|oauth2_cc|hmac|chain);
// all other fields are passed through to the implementation as params.
type AuthSpec struct {
	Type   string         `yaml:"type"`
	Params map[string]any `yaml:",inline"`
}

// UnmarshalYAML compiles concise authentication declarations into the
// runtime strategy descriptor.
func (a *AuthSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode || len(node.Content) != 2 {
		return fmt.Errorf("auth must contain exactly one strategy")
	}
	strategy, value := node.Content[0].Value, node.Content[1]
	a.Params = map[string]any{}
	switch strategy {
	case "none":
		a.Type = ""
	case "bearer":
		a.Type = "bearer"
		if value.Kind != yaml.ScalarNode {
			return fmt.Errorf("auth.bearer must be a reference")
		}
		a.Params["token"] = referenceTemplate(value.Value)
	case "header":
		a.Type = "header"
		var spec struct {
			Name  string `yaml:"name"`
			Value string `yaml:"value"`
		}
		if err := value.Decode(&spec); err != nil {
			return err
		}
		a.Params["name"], a.Params["value"] = spec.Name, referenceTemplate(spec.Value)
	case "basic":
		a.Type = "basic"
		var spec struct {
			Username string `yaml:"username"`
			Password string `yaml:"password"`
		}
		if err := value.Decode(&spec); err != nil {
			return err
		}
		a.Params["user"] = referenceTemplate(spec.Username)
		a.Params["pass"] = referenceTemplate(spec.Password)
	case "oauth2":
		a.Type = "oauth2_cc"
		var spec struct {
			TokenURL     string `yaml:"token_url"`
			ClientID     string `yaml:"client_id"`
			ClientSecret string `yaml:"client_secret"`
			Scope        string `yaml:"scope"`
		}
		if err := value.Decode(&spec); err != nil {
			return err
		}
		a.Params["token_url"] = spec.TokenURL
		a.Params["client_id"] = referenceTemplate(spec.ClientID)
		a.Params["client_secret"] = referenceTemplate(spec.ClientSecret)
		a.Params["scope"] = spec.Scope
	default:
		return fmt.Errorf("unknown auth strategy %q", strategy)
	}
	return nil
}

func referenceTemplate(value string) string {
	if strings.Contains(value, "{{") || !strings.Contains(value, ".") {
		return value
	}
	return "{{ " + value + " }}"
}

// RateLimit configures the request rate ceiling, optionally header-driven.
type RateLimit struct {
	RequestsPerSecond float64       `yaml:"requests_per_second"`
	Dynamic           *DynamicLimit `yaml:"dynamic,omitempty"`
}

// DynamicLimit configures rate adjustment from response rate-limit headers.
type DynamicLimit struct {
	RemainingHeader string  `yaml:"remaining_header"`
	ResetHeader     string  `yaml:"reset_header"`
	ResetFormat     string  `yaml:"reset_format"` // unix_seconds | seconds_from_now | http_date
	MinFloorRPS     float64 `yaml:"min_floor_rps"`
}

// Resource declares one extractable endpoint and how to page, decode, and
// incrementally track it.
type Resource struct {
	Name          string    `yaml:"name"`
	EmitAs        string    `yaml:"emit_as,omitempty"`
	Path          string    `yaml:"path"`
	Method        string    `yaml:"method"`
	Mode          string    `yaml:"mode"` // paginated | stream (default paginated)
	PrimaryKey    []string  `yaml:"primary_key"`
	Fields        FieldList `yaml:"fields,omitempty"`
	UseFields     []string  `yaml:"use_fields,omitempty"`
	ExcludeFields []string  `yaml:"exclude_fields,omitempty"`
	// Records is concise syntax for response.root + response.records_path.
	// "$" means an array at the document root; "$.data.items" selects a path.
	Records string `yaml:"records,omitempty"`
	// ForEach is concise syntax for parent.resource.
	ForEach     string            `yaml:"for_each,omitempty"`
	Params      map[string]string `yaml:"params,omitempty"`
	Capture     map[string]string `yaml:"capture,omitempty"`
	Query       map[string]string `yaml:"query,omitempty"`
	Headers     map[string]string `yaml:"headers,omitempty"`
	Body        BodySpec          `yaml:"body,omitempty"`
	Response    ResponseSpec      `yaml:"response"`
	Pagination  PaginationSpec    `yaml:"pagination"`
	Incremental *IncrementalSpec  `yaml:"incremental,omitempty"`
	Parent      *ParentRef        `yaml:"parent,omitempty"`
	Stream      *StreamSpec       `yaml:"stream,omitempty"`
}

// FieldSpec maps a response path to a typed output field.
type FieldSpec struct {
	Name     string            `yaml:"name"`
	Path     string            `yaml:"path"`
	Type     string            `yaml:"type"`
	Shape    map[string]string `yaml:"shape,omitempty"`
	Mode     string            `yaml:"mode,omitempty"` // raw | remainder (json fields only)
	Nullable bool              `yaml:"nullable,omitempty"`
}

// FieldList is the v1 map-based field declaration. YAML mapping order is
// retained so schemas remain stable and readable.
type FieldList []FieldSpec

// UnmarshalYAML accepts `field: type?` or an expanded value with path, type,
// shape, mode, and nullable. The old list syntax is intentionally rejected.
func (fields *FieldList) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("fields must be a mapping")
	}
	out := make(FieldList, 0, len(node.Content)/2)
	for i := 0; i < len(node.Content); i += 2 {
		name := node.Content[i].Value
		value := node.Content[i+1]
		field := FieldSpec{Name: name, Path: name}
		if value.Kind == yaml.ScalarNode {
			field.Nullable = strings.HasSuffix(value.Value, "?")
			field.Type = strings.TrimSuffix(value.Value, "?")
		} else {
			type valueSpec struct {
				Path     string            `yaml:"path"`
				Type     string            `yaml:"type"`
				Shape    map[string]string `yaml:"shape,omitempty"`
				Mode     string            `yaml:"mode,omitempty"`
				Nullable bool              `yaml:"nullable,omitempty"`
			}
			var spec valueSpec
			if err := value.Decode(&spec); err != nil {
				return fmt.Errorf("field %q: %w", name, err)
			}
			field.Path, field.Type, field.Shape, field.Mode, field.Nullable = spec.Path, spec.Type, spec.Shape, spec.Mode, spec.Nullable
			if field.Path == "" && len(field.Shape) == 0 {
				field.Path = name
			}
		}
		out = append(out, field)
	}
	*fields = out
	return nil
}

// BodySpec configures the request body encoding and template.
type BodySpec struct {
	Encoding string `yaml:"encoding"` // json | form | multipart | raw | none
	Template any    `yaml:"template,omitempty"`
}

// ResponseSpec configures where records live in the response body.
type ResponseSpec struct {
	Root        string          `yaml:"root"` // array | object (default object)
	RecordsPath string          `yaml:"records_path"`
	Cardinality string          `yaml:"cardinality,omitempty"` // many (default) | one
	Error       *ErrorSpec      `yaml:"error,omitempty"`
	Records     string          `yaml:"records,omitempty"`
	Pagination  *PaginationSpec `yaml:"pagination,omitempty"`
}

// ErrorSpec configures detection of errors wrapped in 200 responses.
type ErrorSpec struct {
	Path        string `yaml:"path"`
	WhenPresent bool   `yaml:"when_present"`
	MessagePath string `yaml:"message_path,omitempty"`
	CodePath    string `yaml:"code_path,omitempty"`
}

// PaginationSpec configures how list endpoints are paged.
type PaginationSpec struct {
	Type string `yaml:"type"` // cursor | offset | page | link_header | next_url | none

	// cursor
	CursorPath  string `yaml:"cursor_path,omitempty"`
	CursorParam string `yaml:"cursor_param,omitempty"`
	InjectInto  string `yaml:"inject_into,omitempty"` // body | query | header
	HasMorePath string `yaml:"has_more_path,omitempty"`
	// AllowNullTerminates, when true, treats an explicit JSON null at
	// cursor_path as "no more pages" rather than an error. Default false:
	// null is rejected so that an API silently changing its termination
	// signal from missing to null doesn't masquerade as the end of data.
	// Opt in only when the upstream contract documents null-as-terminator.
	AllowNullTerminates bool `yaml:"allow_null_terminates,omitempty"`

	// offset
	OffsetParam      string `yaml:"offset_param,omitempty"`
	LimitParam       string `yaml:"limit_param,omitempty"`
	PageSize         int    `yaml:"page_size,omitempty"`
	OffsetInjectInto string `yaml:"offset_inject_into,omitempty"`

	// page-number
	PageParam      string `yaml:"page_param,omitempty"`
	SizeParam      string `yaml:"size_param,omitempty"`
	TotalPagesPath string `yaml:"total_pages_path,omitempty"`

	// link_header
	Rel string `yaml:"rel,omitempty"`

	// next_url
	NextURLPath string `yaml:"next_url_path,omitempty"`
}

// UnmarshalYAML accepts concise pagination strategies while retaining a
// strategy-neutral runtime representation.
func (p *PaginationSpec) UnmarshalYAML(node *yaml.Node) error {
	if node.Kind == yaml.ScalarNode {
		if node.Value != "none" {
			return fmt.Errorf("pagination scalar must be none")
		}
		p.Type = "none"
		return nil
	}
	if node.Kind != yaml.MappingNode {
		return fmt.Errorf("pagination must be a strategy mapping")
	}
	// Internal explicit form remains useful to generated test manifests.
	for i := 0; i < len(node.Content); i += 2 {
		if node.Content[i].Value != "type" {
			continue
		}
		type plain PaginationSpec
		var out plain
		if err := node.Decode(&out); err != nil {
			return err
		}
		*p = PaginationSpec(out)
		return nil
	}
	if len(node.Content) != 2 {
		return fmt.Errorf("pagination must contain exactly one strategy")
	}
	strategy, value := node.Content[0].Value, node.Content[1]
	switch strategy {
	case "link":
		p.Type = "link_header"
		return value.Decode(&p.Rel)
	case "next_url":
		p.Type = "next_url"
		return value.Decode(&p.NextURLPath)
	case "cursor":
		var spec struct {
			Response       string `yaml:"response"`
			Request        string `yaml:"request"`
			More           string `yaml:"more"`
			NullTerminates bool   `yaml:"null_terminates"`
		}
		if err := value.Decode(&spec); err != nil {
			return err
		}
		target, param, ok := strings.Cut(spec.Request, ".")
		if !ok || (target != "query" && target != "body" && target != "header") {
			return fmt.Errorf("cursor.request must be query.<name>, body.<path>, or header.<name>")
		}
		p.Type, p.CursorPath, p.InjectInto, p.CursorParam, p.HasMorePath, p.AllowNullTerminates = "cursor", spec.Response, target, param, spec.More, spec.NullTerminates
	case "offset":
		var spec struct {
			Offset   string `yaml:"offset"`
			Limit    string `yaml:"limit"`
			PageSize int    `yaml:"page_size"`
		}
		if err := value.Decode(&spec); err != nil {
			return err
		}
		offsetTarget, offsetParam, ok := strings.Cut(spec.Offset, ".")
		if !ok {
			offsetTarget, offsetParam = "query", spec.Offset
		}
		limitTarget, limitParam, ok := strings.Cut(spec.Limit, ".")
		if !ok {
			limitTarget, limitParam = "query", spec.Limit
		}
		if offsetTarget != limitTarget || (offsetTarget != "query" && offsetTarget != "body") {
			return fmt.Errorf("offset pagination fields must share a query or body target")
		}
		p.Type, p.OffsetParam, p.LimitParam, p.PageSize, p.OffsetInjectInto = "offset", offsetParam, limitParam, spec.PageSize, offsetTarget
	case "page":
		var spec struct {
			Number     string `yaml:"number"`
			Size       string `yaml:"size"`
			PageSize   int    `yaml:"page_size"`
			TotalPages string `yaml:"total_pages"`
		}
		if err := value.Decode(&spec); err != nil {
			return err
		}
		p.Type, p.PageParam, p.SizeParam, p.PageSize, p.TotalPagesPath = "page", spec.Number, spec.Size, spec.PageSize, spec.TotalPages
	default:
		return fmt.Errorf("unknown pagination strategy %q", strategy)
	}
	return nil
}

// IncrementalSpec configures watermark-based incremental extraction.
type IncrementalSpec struct {
	CursorField string `yaml:"cursor_field"`
	// CursorPath is resolved from the projected field declaration after parsing.
	// It is runtime-only; manifests continue to name the output cursor field.
	CursorPath    string `yaml:"-"`
	StartParam    string `yaml:"start_param"`
	InjectInto    string `yaml:"inject_into"` // query | body | header
	Initial       string `yaml:"initial,omitempty"`
	CheckpointKey string `yaml:"checkpoint_key,omitempty"`
	// Comparator selects watermark-advance ordering. lex (default) compares as
	// strings; numeric parses both sides as float64; time parses RFC3339.
	Comparator string `yaml:"comparator,omitempty"` // lex | numeric | time
	// OverlapSeconds re-fetches a sliding window before a time or numeric
	// timestamp cursor to tolerate retroactive updates behind the max.
	OverlapSeconds int `yaml:"overlap_seconds,omitempty"`
}

// ParentRef declares a child resource's dependency on a parent. The parent's
// `capture` block dictates which fields land in the `parent.*` template scope
// of child requests; ParentRef carries no field list of its own.
type ParentRef struct {
	Resource    string `yaml:"resource"`
	Concurrency int    `yaml:"concurrency,omitempty"`
}

// StreamSpec configures streaming-mode decoding of a resource's response.
type StreamSpec struct {
	Type        string `yaml:"type"` // ndjson | sse | chunked_array
	EventFilter string `yaml:"event_filter,omitempty"`
	// ArrayPath (chunked_array only) is a dot-path to the wrapping array
	// inside the response body. Example: `data.items` for `{"data":{"items":[...]}}`.
	// Empty string means the array is at the document root.
	ArrayPath string `yaml:"array_path,omitempty"`
	// MaxElementBytes caps per-element size for chunked_array. Zero means
	// no cap (relies on the connector-level response size cap).
	MaxElementBytes int `yaml:"max_element_bytes,omitempty"`
}

// Load reads, parses, validates, and version-checks a manifest at path.
func Load(path string) (*Manifest, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("read manifest: %w", err)
	}
	return Parse(data)
}

// Parse validates + decodes a YAML manifest byte slice. Two-phase validation:
// grammar (shape + enum membership) before decode, semantics (cross-field
// rules) after. Semantic errors are aggregated — every problem found in one
// pass is reported, not just the first. The returned error wraps
// errs.ErrManifestValidate.
func Parse(data []byte) (*Manifest, error) {
	if err := ValidateGrammar(data); err != nil {
		return nil, fmt.Errorf("manifest grammar: %w", err)
	}
	var m Manifest
	dec := yaml.NewDecoder(bytes.NewReader(data))
	dec.KnownFields(true)
	if err := dec.Decode(&m); err != nil {
		return nil, fmt.Errorf("parse manifest: %w", err)
	}
	if m.Version != SupportedVersion {
		return nil, fmt.Errorf("unsupported manifest version %d (want %d)", m.Version, SupportedVersion)
	}
	m.Normalize()
	if err := m.validateSemantics(); err != nil {
		return nil, err
	}
	return &m, nil
}
