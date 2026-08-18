package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"slices"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/galaxy-io/filament"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament/connectors/http/incremental"
	"github.com/galaxy-io/filament/connectors/http/internal/atomicwatermark"
	"github.com/galaxy-io/filament/connectors/http/manifest"
)

const providerName = "httpapi"

var genericConfig = filament.ConfigSchema{Fields: []filament.ConfigField{
	{Name: "manifest_path", Type: filament.FieldString, Required: true, Scope: filament.ScopeConnection, Help: "Path to a v1 HTTP API connector manifest"},
}}

type selectorToken struct {
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id"`
}

// Source adapts the manifest-driven HTTP connector to ingestion's source API.
type Source struct {
	connector        *Connector
	name             string
	displayName      string
	description      string
	darkLogoURL      string
	lightLogoURL     string
	config           filament.ConfigSchema
	manifestData     []byte
	dynamicResources map[string]string
	// incrementalResources is populated by PlanIncremental for the resources in
	// the current run. It keeps durable watermark extraction distinct from
	// ordinary within-run pagination resume.
	incrementalResources map[string]manifest.IncrementalSpec
	incrementalLookbacks map[string]int
}

var (
	_ filament.Source               = (*Source)(nil)
	_ filament.Discoverable         = (*Source)(nil)
	_ filament.LiveValidatable      = (*Source)(nil)
	_ filament.Resumable            = (*Source)(nil)
	_ filament.ResumePlanner        = (*Source)(nil)
	_ filament.IncrementalPlanner   = (*Source)(nil)
	_ filament.ResourcePlanner      = (*Source)(nil)
	_ filament.SchemaProvider       = (*Source)(nil)
	_ filament.CursorColumnProvider = (*Source)(nil)
)

// New returns the generic manifest-path-configured HTTP source.
func New() *Source {
	return &Source{name: providerName, displayName: "HTTP API", config: genericConfig}
}

// NewManifest returns a Source bound to embedded manifest bytes and a config schema.
func NewManifest(name, displayName string, manifestData []byte, config filament.ConfigSchema) *Source {
	return NewManifestWithMetadata(name, displayName, "", "", "", manifestData, config)
}

// NewManifestWithMetadata returns a Source bound to embedded manifest bytes,
// frontend catalog metadata, and a config schema.
func NewManifestWithMetadata(name, displayName, description, darkLogoURL, lightLogoURL string, manifestData []byte, config filament.ConfigSchema) *Source {
	return &Source{name: name, displayName: displayName, description: description, darkLogoURL: darkLogoURL, lightLogoURL: lightLogoURL, manifestData: manifestData, config: config}
}

// Spec reports the source's capabilities and configuration surface.
func (s *Source) Spec() filament.ConnectorSpec {
	config := s.configSchema()
	modes := []filament.ReadMode{filament.ModeFull}
	policies := []filament.IngestionType{
		filament.IngestionFullReplace,
		filament.IngestionFullUpsert,
		filament.IngestionFullAppend,
	}
	if s.canDeclareIncremental() {
		modes = append(modes, filament.ModeIncremental)
		policies = append(policies,
			filament.IngestionIncrementalAppend,
			filament.IngestionIncrementalUpsert,
		)
	}
	return filament.ConnectorSpec{
		Name:           s.name,
		DisplayName:    s.displayName,
		Description:    s.description,
		DarkLogoURL:    s.darkLogoURL,
		LightLogoURL:   s.lightLogoURL,
		Version:        "1",
		Modes:          modes,
		SourcePolicies: filament.SourcePolicies(policies...),
		Config:         config,
		Resources:      filament.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}

// canDeclareIncremental is optimistic for the generic manifest-path source,
// whose manifest is unavailable until configuration. Embedded connectors only
// advertise incremental mode when at least one resource actually declares a
// durable watermark.
func (s *Source) canDeclareIncremental() bool {
	if s.connector != nil && s.connector.manifest != nil {
		for _, resource := range s.connector.manifest.Resources {
			if resource.Incremental != nil {
				return true
			}
		}
		return false
	}
	if len(s.manifestData) == 0 {
		return true
	}
	m, err := manifest.Parse(s.manifestData)
	if err != nil {
		return false
	}
	for _, resource := range m.Resources {
		if resource.Incremental != nil {
			return true
		}
	}
	return false
}

// Validate checks that all required config fields are present and non-empty.
func (s *Source) Validate(cfg filament.Config) error {
	for _, field := range s.configSchema().Fields {
		if field.Required && !cfg.Has(field.Name) {
			return fmt.Errorf("%s source: %s is required", s.name, field.Name)
		}
		if field.Required && field.Type == filament.FieldString && cfg.String(field.Name) == "" {
			return fmt.Errorf("%s source: %s is required", s.name, field.Name)
		}
		if field.Required && field.Type == filament.FieldSecret && cfg.Secret(field.Name) == "" {
			return fmt.Errorf("%s source: %s is required", s.name, field.Name)
		}
		if field.Required && field.Type == filament.FieldList && listLen(cfg.Raw()[field.Name]) == 0 {
			return fmt.Errorf("%s source: %s is required", s.name, field.Name)
		}
	}
	return nil
}

func listLen(value any) int {
	switch value := value.(type) {
	case []string:
		return len(value)
	case []any:
		return len(value)
	case string: // Backward compatibility for previously comma-separated values.
		count := 0
		for _, item := range strings.Split(value, ",") {
			if strings.TrimSpace(item) != "" {
				count++
			}
		}
		return count
	default:
		return 0
	}
}

func (s *Source) configSchema() filament.ConfigSchema {
	if len(s.manifestData) == 0 {
		return s.config
	}
	m, err := manifest.Parse(s.manifestData)
	if err != nil || len(m.Config) == 0 {
		return s.config
	}
	names := make([]string, 0, len(m.Config))
	for name := range m.Config {
		names = append(names, name)
	}
	sort.Strings(names)
	fields := make([]filament.ConfigField, 0, len(names))
	for _, name := range names {
		spec := m.Config[name]
		fieldType := filament.FieldString
		switch spec.Type {
		case "int":
			fieldType = filament.FieldInt
		case "bool":
			fieldType = filament.FieldBool
		case "duration":
			fieldType = filament.FieldDuration
		case "enum":
			fieldType = filament.FieldEnum
		case "object":
			fieldType = filament.FieldObject
		case "list":
			fieldType = filament.FieldList
		case "secret":
			fieldType = filament.FieldSecret
		}
		scope := filament.ScopeConnection
		if spec.Scope == "pipeline" {
			scope = filament.ScopePipeline
		}
		options := make([]filament.EnumOption, len(spec.Enum))
		for i, value := range spec.Enum {
			options[i] = filament.EnumOption{Value: value, Label: value}
		}
		fields = append(fields, filament.ConfigField{
			Name: name, Type: fieldType, Required: spec.Required, Default: spec.Default,
			Enum: options, Help: spec.Help, Scope: scope, Secret: spec.Type == "secret",
		})
	}
	return filament.ConfigSchema{Fields: fields}
}

// Configure validates cfg and builds the underlying connector.
func (s *Source) Configure(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	c, err := s.connectorForConfig(cfg)
	if err != nil {
		return err
	}
	if err := c.Validate(); err != nil {
		return fmt.Errorf("httpapi source: validate connector: %w", err)
	}
	if err := c.Configure(ctx); err != nil {
		return fmt.Errorf("httpapi source: configure connector: %w", err)
	}
	s.connector = c
	return nil
}

// TestConnection builds the manifest connector and performs a single,
// authenticated API request without extracting or persisting any records.
func (s *Source) TestConnection(ctx context.Context, cfg filament.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	c, err := s.connectorForConfig(cfg)
	if err != nil {
		return err
	}
	if err := c.Configure(ctx); err != nil {
		return fmt.Errorf("%s source: configure connection probe: %w", s.name, err)
	}
	defer func() { _ = c.Teardown(ctx) }()
	if err := c.TestConnection(ctx); err != nil {
		return fmt.Errorf("%s source: %w", s.name, err)
	}
	return nil
}

func (s *Source) connectorForConfig(cfg filament.Config) (*Connector, error) {
	c := &Connector{}
	if len(s.manifestData) > 0 {
		c.SetManifestData(s.manifestData)
	} else {
		c.SetManifestPath(cfg.String("manifest_path"))
	}
	var configSpecs map[string]manifest.ConfigSpec
	if len(s.manifestData) > 0 {
		parsed, err := manifest.Parse(s.manifestData)
		if err != nil {
			return nil, fmt.Errorf("%s source: parse manifest: %w", s.name, err)
		}
		configSpecs = parsed.Config
	} else {
		parsed, err := manifest.Load(cfg.String("manifest_path"))
		if err != nil {
			return nil, fmt.Errorf("%s source: load manifest: %w", s.name, err)
		}
		configSpecs = parsed.Config
	}
	c.SetCredentials(credentialsFromConfig(cfg, configSpecs))
	return c, nil
}

// Discover enumerates selectable resources, falling back to the manifest's
// static resource list when the manifest declares no discovery spec.
func (s *Source) Discover(ctx context.Context, _ filament.DiscoverOpts) (filament.DiscoverResult, error) {
	if s.connector == nil || s.connector.manifest == nil {
		return filament.DiscoverResult{}, fmt.Errorf("httpapi source: discover before configure")
	}
	res, err := s.connector.Discover(ctx)
	if err != nil {
		return filament.DiscoverResult{}, err
	}
	if len(res.Resources) == 0 {
		staticResources := s.connector.manifest.Resources
		if include := s.connector.manifest.Discovery.Include; len(include) > 0 {
			byName := make(map[string]manifest.Resource, len(staticResources))
			for _, resource := range staticResources {
				byName[resource.Name] = resource
			}
			staticResources = make([]manifest.Resource, 0, len(include))
			for _, name := range include {
				staticResources = append(staticResources, byName[name])
			}
		}
		out := make([]filament.Resource, 0, len(staticResources))
		for _, r := range staticResources {
			schema, _ := s.Schema(ctx, r.Name)
			out = append(out, filament.Resource{
				Name:       r.Name,
				Selector:   r.Name,
				Selectable: true,
				PrimaryKey: append([]string(nil), r.PrimaryKey...),
				Schema:     &schema,
			})
		}
		return filament.DiscoverResult{Resources: out}, nil
	}
	return res, nil
}

// Extract runs a full extraction into sink.
func (s *Source) Extract(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts) error {
	s.incrementalResources = nil
	s.incrementalLookbacks = nil
	return s.extract(ctx, sink, opts, nil, nil)
}

// ExtractFrom resumes extraction from per-resource keyset checkpoints,
// decoding them into resume cursors and watermarks.
func (s *Source) ExtractFrom(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts, prev map[string]filament.Checkpoint) error {
	resumeCursors := make(map[string]string, len(prev))
	resumeWatermarks := make(map[string]map[string]string, len(prev))
	for resource, cp := range prev {
		ks, ok := checkpoint.ParseKeyset(cp)
		if !ok || len(ks.Shards) == 0 || len(ks.Shards[0].Key) == 0 {
			continue
		}
		key := ks.Shards[0].Key
		if ks.Mode == checkpoint.ModeIncremental {
			for i, checkpointKey := range ks.Cols {
				if i >= len(key) || key[i] == "" {
					continue
				}
				if resumeWatermarks[resource] == nil {
					resumeWatermarks[resource] = map[string]string{}
				}
				resumeWatermarks[resource][checkpointKey] = key[i]
			}
			continue
		}
		resumeCursors[resource] = key[0]
		for i, checkpointKey := range ks.Cols[1:] {
			if i+1 >= len(key) || key[i+1] == "" {
				continue
			}
			if resumeWatermarks[resource] == nil {
				resumeWatermarks[resource] = map[string]string{}
			}
			resumeWatermarks[resource][checkpointKey] = key[i+1]
		}
	}
	return s.extract(ctx, sink, opts, resumeCursors, resumeWatermarks)
}

// PlanResources resolves requested resources and selectors to manifest resource names.
func (s *Source) PlanResources(_ context.Context, resources, selectors []string) ([]string, error) {
	if s.connector == nil || s.connector.manifest == nil {
		return nil, fmt.Errorf("httpapi source: plan resources before configure")
	}
	return s.planResources(resources, selectors)
}

// PlanResume builds per-resource pagination checkpoints for a full read.
// Durable watermarks belong exclusively to PlanIncremental.
func (s *Source) PlanResume(_ context.Context, resources []string, prev map[string]filament.Checkpoint) (map[string]filament.Checkpoint, error) {
	if s.connector == nil || s.connector.manifest == nil {
		return nil, fmt.Errorf("httpapi source: plan resume before configure")
	}
	resources, err := s.planResources(resources, nil)
	if err != nil {
		return nil, err
	}
	plan := make(map[string]filament.Checkpoint, len(resources))
	for _, resource := range resources {
		cols := []string{"cursor"}
		types := make([]string, len(cols))
		for i := range types {
			types[i] = "string"
		}
		if ks, ok := checkpoint.ParseKeyset(prev[resource]); ok && len(ks.Shards) > 0 {
			ks.Cols = cols
			ks.Types = types
			if len(ks.Shards[0].Key) > len(cols) {
				ks.Shards[0].Key = ks.Shards[0].Key[:len(cols)]
			}
			plan[resource] = ks.ToCheckpoint(resource)
			continue
		}
		plan[resource] = checkpoint.KeysetCheckpoint{
			Cols:   cols,
			Types:  types,
			Shards: []checkpoint.KeysetShard{{}},
		}.ToCheckpoint(resource)
	}
	return plan, nil
}

// Schema returns the declared record schema for a resource.
func (s *Source) Schema(_ context.Context, resource string) (filament.RecordSchema, error) {
	if s.connector == nil || s.connector.manifest == nil {
		return filament.RecordSchema{}, fmt.Errorf("httpapi source: schema before configure")
	}
	base := s.baseResourceName(resource)
	for _, res := range s.connector.manifest.Resources {
		if res.Name != base {
			continue
		}
		return filament.RecordSchema{Resource: resource, Fields: schemaFields(res), PrimaryKey: append([]string(nil), res.PrimaryKey...)}, nil
	}
	return filament.RecordSchema{}, fmt.Errorf("httpapi source: unknown resource %q", resource)
}

func (s *Source) extract(ctx context.Context, sink filament.RecordSink, opts filament.ExtractOpts, resumeCursors map[string]string, resumeWatermarks map[string]map[string]string) error {
	if s.connector == nil || s.connector.manifest == nil {
		return fmt.Errorf("httpapi source: extract before configure")
	}

	reducingSink := &incrementalRecordSink{
		sink:    sink,
		reducer: newIncrementalRecordReducer(s, resumeWatermarks),
	}
	return s.connector.Extract(ctx, reducingSink, extractOptions{
		Observe:              opts.Observe,
		Resources:            s.connectorResources(opts.Resources),
		EnabledResources:     enabledResources(opts.Selectors),
		ResumeCursors:        resumeCursors,
		ResumeWatermarks:     resumeWatermarks,
		IncrementalLookbacks: s.incrementalLookbacks,
		IncrementalResources: s.incrementalResourceSet(),
	})
}

func (s *Source) incrementalResourceSet() map[string]bool {
	out := make(map[string]bool, len(s.incrementalResources))
	for resource := range s.incrementalResources {
		out[resource] = true
	}
	return out
}

func (s *Source) connectorResources(resources []string) []string {
	if len(resources) == 0 {
		return nil
	}
	out := make([]string, 0, len(resources))
	seen := map[string]struct{}{}
	for _, resource := range resources {
		base := s.baseResourceName(resource)
		if _, ok := seen[base]; ok {
			continue
		}
		seen[base] = struct{}{}
		out = append(out, base)
	}
	return out
}

func (s *Source) planResources(resources, selectors []string) ([]string, error) {
	if len(resources) > 0 && len(selectors) == 0 {
		return resources, nil
	}
	// Static discovery uses the resource name itself as its selector. Only
	// encoded selectors carry dynamic discovery semantics; otherwise a single
	// static edge would fall through below and plan every manifest resource.
	hasDynamicSelector := false
	for _, selector := range selectors {
		if _, ok := decodeSelector(selector); ok {
			hasDynamicSelector = true
			break
		}
	}
	if len(resources) > 0 && !hasDynamicSelector {
		return resources, nil
	}
	sorted := manifest.SortResources(s.connector.manifest.Resources)
	out := make([]string, 0, len(sorted)+len(selectors))
	seen := map[string]struct{}{}
	add := func(resource string) {
		if resource == "" {
			return
		}
		if _, ok := seen[resource]; ok {
			return
		}
		seen[resource] = struct{}{}
		out = append(out, resource)
	}
	s.dynamicResources = map[string]string{}
	if len(selectors) == 0 {
		for _, res := range sorted {
			add(res.Name)
		}
		return out, nil
	}
	refs := enabledResources(selectors)
	for _, res := range sorted {
		if res.Parent == nil || res.EmitAs == "" {
			add(res.Name)
			continue
		}
		for _, parent := range s.syntheticParentsFor(res, refs) {
			name, err := emittedResourceName(res, parent)
			if err != nil {
				return nil, err
			}
			add(name)
			s.dynamicResources[name] = res.Name
		}
	}
	return out, nil
}

func (s *Source) syntheticParentsFor(res manifest.Resource, refs []resourceRef) []Capture {
	if res.Parent == nil {
		return nil
	}
	parent, ok := s.manifestResource(res.Parent.Resource)
	if !ok {
		return nil
	}
	var out []Capture
	for _, disc := range s.connector.manifest.Discovery.Resources {
		if disc.From != parent.Name {
			continue
		}
		for _, ref := range refs {
			if ref.Kind != "" && ref.Kind != disc.Map.Kind {
				continue
			}
			parentCapture := Capture{"id": ref.ID}
			for field, path := range parent.Capture {
				if path == disc.Map.IDPath {
					parentCapture[field] = ref.ID
				}
			}
			out = append(out, parentCapture)
		}
	}
	return out
}

func (s *Source) manifestResource(name string) (manifest.Resource, bool) {
	for _, res := range s.connector.manifest.Resources {
		if res.Name == name {
			return res, true
		}
	}
	return manifest.Resource{}, false
}

func (s *Source) baseResourceName(resource string) string {
	if base := s.dynamicResources[resource]; base != "" {
		return base
	}
	for _, res := range s.connector.manifest.Resources {
		if res.Name == resource {
			return resource
		}
		if res.EmitAs == "" {
			continue
		}
		if prefix := dynamicNamePrefix(res.EmitAs); prefix != "" && strings.HasPrefix(resource, prefix) {
			return res.Name
		}
	}
	return resource
}

func dynamicNamePrefix(tmpl string) string {
	if i := strings.Index(tmpl, "{{"); i >= 0 {
		return tmpl[:i]
	}
	return tmpl
}

// Teardown releases the underlying connector's resources.
func (s *Source) Teardown(ctx context.Context) error {
	if s.connector == nil {
		return nil
	}
	return s.connector.Teardown(ctx)
}

func credentialsFromConfig(cfg filament.Config, specs map[string]manifest.ConfigSpec) map[string]string {
	creds := map[string]string{}
	values := make(map[string]any, len(cfg.Raw())+len(specs))
	for key, value := range cfg.Raw() {
		values[key] = value
	}
	for name, spec := range specs {
		if _, exists := values[name]; !exists && spec.Default != nil {
			values[name] = spec.Default
		}
	}
	for k, v := range values {
		if k == "manifest_path" {
			continue
		}
		switch value := v.(type) {
		case string:
			creds[k] = value
		case fmt.Stringer:
			creds[k] = value.String()
		case int:
			creds[k] = strconv.Itoa(value)
		case int64:
			creds[k] = strconv.FormatInt(value, 10)
		case float64:
			creds[k] = strconv.FormatFloat(value, 'f', -1, 64)
		case bool:
			creds[k] = strconv.FormatBool(value)
		case []string:
			creds[k] = strings.Join(value, ",")
		case []any:
			items := make([]string, 0, len(value))
			for _, item := range value {
				if text, ok := item.(string); ok {
					items = append(items, text)
				}
			}
			creds[k] = strings.Join(items, ",")
		}
	}
	return creds
}

func enabledResources(selectors []string) []resourceRef {
	if len(selectors) == 0 {
		return nil
	}
	out := make([]resourceRef, 0, len(selectors))
	for _, selector := range selectors {
		ref, ok := decodeSelector(selector)
		if !ok {
			ref.ID = selector
		}
		out = append(out, ref)
	}
	return out
}

func encodeSelector(kind, id string) string {
	if id == "" {
		return ""
	}
	b, err := json.Marshal(selectorToken{Kind: kind, ID: id})
	if err != nil {
		return id
	}
	return string(b)
}

func decodeSelector(selector string) (resourceRef, bool) {
	var token selectorToken
	if err := json.Unmarshal([]byte(selector), &token); err != nil || token.ID == "" {
		return resourceRef{}, false
	}
	return resourceRef{Kind: token.Kind, ID: token.ID}, true
}

func newHTTPRecord(resource string, keyJSON, dataJSON []byte, projected bool) filament.Record {
	out := filament.Record{
		Resource: resource,
		ID:       recordID(keyJSON),
		Op:       filament.OpInsert,
		Data:     dataJSON,
	}
	if !projected {
		out.Data = recordData(keyJSON, dataJSON)
	}
	return out
}

func recordData(keyJSON, dataJSON []byte) []byte {
	var key map[string]any
	if err := json.Unmarshal(keyJSON, &key); err != nil {
		return dataJSON
	}
	var payload json.RawMessage
	if err := json.Unmarshal(dataJSON, &payload); err != nil {
		return dataJSON
	}
	out := make(map[string]any, len(key)+1)
	for k, v := range key {
		out[k] = v
	}
	out["data"] = payload
	b, err := json.Marshal(out)
	if err != nil {
		return dataJSON
	}
	return b
}

func recordID(keyJSON []byte) string {
	if len(keyJSON) == 0 {
		return ""
	}
	var key map[string]any
	if err := json.Unmarshal(keyJSON, &key); err != nil || len(key) != 1 {
		return string(keyJSON)
	}
	for _, v := range key {
		switch value := v.(type) {
		case string:
			return value
		case float64:
			return strconv.FormatFloat(value, 'f', -1, 64)
		case bool:
			return strconv.FormatBool(value)
		default:
			b, err := json.Marshal(value)
			if err != nil {
				return string(keyJSON)
			}
			return string(b)
		}
	}
	return string(keyJSON)
}

// CursorColumns exposes the manifest-declared watermark as the one durable
// cursor for a resource. HTTP cursor selection is declarative rather than
// inferred: changing it requires changing the manifest contract.
func (s *Source) CursorColumns(_ context.Context, resource string) ([]filament.CursorColumn, error) {
	if s.connector == nil || s.connector.manifest == nil {
		return nil, fmt.Errorf("httpapi source: cursor columns before configure")
	}
	res, ok := s.manifestResource(s.baseResourceName(resource))
	if !ok {
		return nil, fmt.Errorf("httpapi source: unknown resource %q", resource)
	}
	if res.Incremental == nil {
		return nil, nil
	}
	field, ok := incrementalField(res)
	if !ok {
		return nil, fmt.Errorf("httpapi source: incremental %q cursor field %q is not projected", resource, res.Incremental.CursorField)
	}
	return []filament.CursorColumn{{
		SchemaField: filament.SchemaField{
			Name: field.Name, Nullable: field.Nullable,
			Logical: logicalType(field.Type), Native: field.Type,
		},
		PrimaryKey:       slices.Contains(res.PrimaryKey, field.Name),
		Eligible:         true,
		Recommended:      true,
		Rank:             1,
		Configurable:     false,
		SupportsLookback: res.Incremental.Comparator == "time" || res.Incremental.Comparator == "numeric",
		Warning:          incrementalCursorWarning(res),
	}}, nil
}

// PlanIncremental creates a watermark-only checkpoint. Pagination cursors are
// deliberately excluded: they are valid for resuming a failed page walk, but
// carrying one into the next scheduled run alongside a newer watermark can
// skip records.
func (s *Source) PlanIncremental(_ context.Context, resources []string, prev map[string]filament.Checkpoint, cursors map[string]filament.ResourceCursorConfig) (map[string]filament.Checkpoint, error) {
	if s.connector == nil || s.connector.manifest == nil {
		return nil, fmt.Errorf("httpapi source: plan incremental before configure")
	}
	planned, err := s.planResources(resources, nil)
	if err != nil {
		return nil, err
	}
	s.incrementalResources = make(map[string]manifest.IncrementalSpec, len(planned))
	lookbacks := make(map[string]int, len(planned))
	plan := make(map[string]filament.Checkpoint, len(planned))
	for _, resource := range planned {
		base := s.baseResourceName(resource)
		res, ok := s.manifestResource(base)
		if !ok {
			return nil, fmt.Errorf("httpapi source: unknown incremental resource %q", resource)
		}
		if res.Incremental == nil {
			return nil, fmt.Errorf("httpapi source: resource %q has no incremental watermark in its manifest", resource)
		}
		field, ok := incrementalField(res)
		if !ok {
			return nil, fmt.Errorf("httpapi source: incremental %q cursor field %q is not projected", resource, res.Incremental.CursorField)
		}
		config, exactConfig := cursors[resource]
		baseConfig := cursors[base]
		if !exactConfig {
			config = baseConfig
		} else if config.Field == "" {
			config.Field = baseConfig.Field
		}
		if config.Field != "" && !strings.EqualFold(config.Field, field.Name) {
			return nil, fmt.Errorf("httpapi source: incremental %q cursor is fixed by the manifest as %q, got %q", resource, field.Name, config.Field)
		}
		spec := *res.Incremental
		if config.LookbackSeconds < 0 {
			return nil, fmt.Errorf("httpapi source: incremental %q lookback must be non-negative", resource)
		}
		if config.LookbackSeconds > 0 {
			if spec.Comparator != "time" && spec.Comparator != "numeric" {
				return nil, fmt.Errorf("httpapi source: incremental %q lookback requires comparator time or numeric", resource)
			}
			spec.OverlapSeconds = int(config.LookbackSeconds)
		}
		s.incrementalResources[resource] = spec
		lookbacks[resource] = spec.OverlapSeconds

		checkpointKey := incremental.CheckpointKey(spec)
		cols := []string{checkpointKey}
		types := []string{field.Type}
		seed := spec.Initial
		if old, ok := checkpoint.ParseKeyset(prev[resource]); ok && len(old.Shards) > 0 {
			if old.Mode == checkpoint.ModeIncremental && slices.Equal(old.Cols, cols) && len(old.Shards) == 1 {
				old.Types = types
				plan[resource] = old.ToCheckpoint(resource)
				continue
			}
		}
		plan[resource] = checkpoint.KeysetCheckpoint{
			Mode: checkpoint.ModeIncremental, Cols: cols, Types: types,
			Shards: []checkpoint.KeysetShard{{Key: watermarkKey(seed)}},
		}.ToCheckpoint(resource)
	}
	s.incrementalLookbacks = lookbacks
	return plan, nil
}

func incrementalField(resource manifest.Resource) (manifest.FieldSpec, bool) {
	if resource.Incremental == nil {
		return manifest.FieldSpec{}, false
	}
	for _, field := range resource.Fields {
		if field.Name == resource.Incremental.CursorField {
			return field, true
		}
	}
	return manifest.FieldSpec{}, false
}

func watermarkKey(value string) []string {
	if value == "" {
		return nil
	}
	return []string{value}
}

func incrementalCursorWarning(resource manifest.Resource) string {
	var warnings []string
	if resource.Incremental.Comparator == "lex" || resource.Incremental.Comparator == "" {
		warnings = append(warnings, "Lexical cursors must have a representation whose byte ordering matches source ordering")
	}
	if resource.Parent != nil {
		warnings = append(warnings, "Fan-out resources use one aggregate watermark; configure a lookback large enough to cover late arrivals during extraction")
	}
	return strings.Join(warnings, "; ")
}

// incrementalRecordReducer makes record keys monotonic even when a manifest
// resource internally fans out concurrent HTTP requests. The outer pipeline is
// ordered, so persisting the last emitted key then persists the maximum value
// observed before it.
type incrementalRecordReducer struct {
	source *Source
	seeds  map[string]map[string]string
	marks  map[string]*atomicwatermark.Watermark
}

func newIncrementalRecordReducer(source *Source, seeds map[string]map[string]string) *incrementalRecordReducer {
	return &incrementalRecordReducer{source: source, seeds: seeds, marks: map[string]*atomicwatermark.Watermark{}}
}

func (r *incrementalRecordReducer) record(rec filament.Record) (filament.Record, error) {
	spec, ok := r.source.incrementalResources[rec.Resource]
	if !ok {
		base := r.source.baseResourceName(rec.Resource)
		spec, ok = r.source.incrementalResources[base]
	}
	if !ok {
		return rec, nil
	}
	checkpointKey := incremental.CheckpointKey(spec)
	mark := r.marks[rec.Resource]
	if mark == nil {
		cmp, err := atomicwatermark.ForName(spec.Comparator)
		if err != nil {
			return filament.Record{}, err
		}
		seed := r.seeds[rec.Resource][checkpointKey]
		if seed == "" {
			seed = r.seeds[r.source.baseResourceName(rec.Resource)][checkpointKey]
		}
		mark, err = atomicwatermark.New(cmp, seed)
		if err != nil {
			return filament.Record{}, err
		}
		r.marks[rec.Resource] = mark
	}
	if value := firstKey(rec.Key); value != "" {
		if _, err := mark.Observe(value); err != nil {
			return filament.Record{}, fmt.Errorf("incremental %q watermark: %w", rec.Resource, err)
		}
	}
	rec.Key = watermarkKey(mark.Current())
	return rec, nil
}

func firstKey(key []string) string {
	if len(key) == 0 {
		return ""
	}
	return key[0]
}

type incrementalRecordSink struct {
	mu      sync.Mutex
	sink    filament.RecordSink
	reducer *incrementalRecordReducer
}

func (s *incrementalRecordSink) Push(rec filament.Record) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	converted, err := s.reducer.record(rec)
	if err != nil {
		return err
	}
	return s.sink.Push(converted)
}

func (s *incrementalRecordSink) PushBatch(records []filament.Record) error {
	for _, rec := range records {
		if err := s.Push(rec); err != nil {
			return err
		}
	}
	return nil
}
