// Package manifest defines the v2 YAML connector manifest schema and loader
// for the generic HTTP connector. A manifest describes the connection,
// authentication, rate limits, and a set of resources (endpoints) to extract.
//
// A manifest MUST set `version: 2`. Any other value is rejected.
//
// # Worked example
//
// Minimal Notion-style cursor-paginated manifest:
//
//	version: 2
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
//   - Wrong version                 → "unsupported manifest version N (want 2)"
//   - Required-field/template/cycle → wrapped errs.ErrManifestValidate with
//     all issues listed (see validateSemantics)
package manifest

import (
	"bytes"
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

const SupportedVersion = 2

type Manifest struct {
	Version    int        `yaml:"version"`
	Name       string     `yaml:"name"`
	Connection Connection `yaml:"connection"`
	Resources  []Resource `yaml:"resources"`
	// Discovery, when set, lets the connector enumerate user-toggleable
	// resources by reusing existing extraction streams. Each entry projects a
	// distinct resource Kind (e.g. notion exposes both "database" and "page"
	// as togglable). Enabled selections are then injected back into other
	// streams via each entry's Scope.
	Discovery []Discovery `yaml:"discovery,omitempty"`
}

// Discovery declares how to enumerate togglable resources from a manifest. It
// reuses an existing Resource (named by From) as the discovery query, so no
// new HTTP plumbing is required.
type Discovery struct {
	From  string      `yaml:"from"`  // name of the Resource that lists togglable items
	Map   ResourceMap `yaml:"map"`   // projection from a record to a pipeline.Resource
	Scope ScopeSpec   `yaml:"scope"` // how enabled selections filter extraction
}

// ResourceMap projects a JSON record (from the Discovery.From stream) to the
// fields of a pipeline.Resource. Paths are JSONPath-style ($.foo.bar).
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

type RateLimit struct {
	RequestsPerSecond float64       `yaml:"requests_per_second"`
	Dynamic           *DynamicLimit `yaml:"dynamic,omitempty"`
}

type DynamicLimit struct {
	RemainingHeader string  `yaml:"remaining_header"`
	ResetHeader     string  `yaml:"reset_header"`
	ResetFormat     string  `yaml:"reset_format"` // unix_seconds | seconds_from_now | http_date
	MinFloorRPS     float64 `yaml:"min_floor_rps"`
}

type Resource struct {
	Name        string            `yaml:"name"`
	EmitAs      string            `yaml:"emit_as,omitempty"`
	Path        string            `yaml:"path"`
	Method      string            `yaml:"method"`
	Mode        string            `yaml:"mode"` // paginated | stream (default paginated)
	PrimaryKey  []string          `yaml:"primary_key"`
	Fields      []FieldSpec       `yaml:"fields,omitempty"`
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

type FieldSpec struct {
	Name     string            `yaml:"name"`
	Path     string            `yaml:"path"`
	Type     string            `yaml:"type"`
	Shape    map[string]string `yaml:"shape,omitempty"`
	Mode     string            `yaml:"mode,omitempty"` // raw | remainder (json fields only)
	Nullable bool              `yaml:"nullable,omitempty"`
}

type BodySpec struct {
	Encoding string `yaml:"encoding"` // json | form | multipart | raw | none
	Template any    `yaml:"template,omitempty"`
}

type ResponseSpec struct {
	Root        string     `yaml:"root"` // array | object (default object)
	RecordsPath string     `yaml:"records_path"`
	Error       *ErrorSpec `yaml:"error,omitempty"`
}

type ErrorSpec struct {
	Path        string `yaml:"path"`
	WhenPresent bool   `yaml:"when_present"`
	MessagePath string `yaml:"message_path,omitempty"`
	CodePath    string `yaml:"code_path,omitempty"`
}

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
	OffsetParam string `yaml:"offset_param,omitempty"`
	LimitParam  string `yaml:"limit_param,omitempty"`
	PageSize    int    `yaml:"page_size,omitempty"`

	// page-number
	PageParam      string `yaml:"page_param,omitempty"`
	SizeParam      string `yaml:"size_param,omitempty"`
	TotalPagesPath string `yaml:"total_pages_path,omitempty"`

	// link_header
	Rel string `yaml:"rel,omitempty"`

	// next_url
	NextURLPath string `yaml:"next_url_path,omitempty"`
}

type IncrementalSpec struct {
	CursorField   string `yaml:"cursor_field"`
	StartParam    string `yaml:"start_param"`
	InjectInto    string `yaml:"inject_into"` // query | body | header
	Initial       string `yaml:"initial,omitempty"`
	CheckpointKey string `yaml:"checkpoint_key,omitempty"`
	// Comparator selects watermark-advance ordering. lex (default) compares as
	// strings; numeric parses both sides as float64; time parses RFC3339.
	Comparator string `yaml:"comparator,omitempty"` // lex | numeric | time
	// OverlapSeconds re-fetches a sliding window before the persisted cursor
	// to tolerate retroactive updates whose timestamps fall behind the max.
	OverlapSeconds int `yaml:"overlap_seconds,omitempty"`
}

// ParentRef declares a child resource's dependency on a parent. The parent's
// `capture` block dictates which fields land in the `parent.*` template scope
// of child requests; ParentRef carries no field list of its own.
type ParentRef struct {
	Resource    string `yaml:"resource"`
	Concurrency int    `yaml:"concurrency,omitempty"`
}

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
