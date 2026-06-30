package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"sync"

	"github.com/galaxy-io/filament/connectors/http/incremental"
	"github.com/galaxy-io/filament/connectors/http/internal/pipeline"
	"github.com/galaxy-io/filament/connectors/http/manifest"
	"github.com/galaxy-io/filament/checkpoint"
	"github.com/galaxy-io/filament"
)

const providerName = "httpapi"

var genericConfig = ingestion.ConfigSchema{Fields: []ingestion.ConfigField{
	{Name: "manifest_path", Type: ingestion.FieldString, Required: true, Help: "Path to a v2 HTTP API connector manifest"},
}}

type selectorToken struct {
	Kind string `json:"kind,omitempty"`
	ID   string `json:"id"`
}

// Source adapts the manifest-driven HTTP connector to ingestion's source API.
type Source struct {
	connector        Connector
	name             string
	displayName      string
	config           ingestion.ConfigSchema
	manifestData     []byte
	dynamicResources map[string]string
}

var (
	_ ingestion.Source          = (*Source)(nil)
	_ ingestion.Discoverable    = (*Source)(nil)
	_ ingestion.Resumable       = (*Source)(nil)
	_ ingestion.ResumePlanner   = (*Source)(nil)
	_ ingestion.ResourcePlanner = (*Source)(nil)
	_ ingestion.SchemaProvider  = (*Source)(nil)
)

func New() *Source {
	return &Source{name: providerName, displayName: "HTTP API", config: genericConfig}
}

func NewManifest(name, displayName string, manifestData []byte, config ingestion.ConfigSchema) *Source {
	return &Source{name: name, displayName: displayName, manifestData: manifestData, config: config}
}

func (s *Source) Spec() ingestion.ConnectorSpec {
	return ingestion.ConnectorSpec{
		Name:        s.name,
		DisplayName: s.displayName,
		Version:     "1",
		Modes:       []ingestion.ReplicationMode{ingestion.ModeFull, ingestion.ModeIncremental},
		SourcePolicies: ingestion.SourcePolicies(
			ingestion.IngestionSnapshotReplace,
			ingestion.IngestionSnapshotUpsert,
			ingestion.IngestionAppend,
		),
		Config:    s.config,
		Resources: ingestion.ResourceCapabilities{Discoverable: true, PerResourceCursor: true},
	}
}

func (s *Source) Validate(cfg ingestion.Config) error {
	for _, field := range s.config.Fields {
		if field.Required && !cfg.Has(field.Name) {
			return fmt.Errorf("%s source: %s is required", s.name, field.Name)
		}
		if field.Required && field.Type == ingestion.FieldString && cfg.String(field.Name) == "" {
			return fmt.Errorf("%s source: %s is required", s.name, field.Name)
		}
		if field.Required && field.Type == ingestion.FieldSecret && cfg.Secret(field.Name) == "" {
			return fmt.Errorf("%s source: %s is required", s.name, field.Name)
		}
	}
	return nil
}

func (s *Source) Configure(ctx context.Context, cfg ingestion.Config) error {
	if err := s.Validate(cfg); err != nil {
		return err
	}
	c := Connector{}
	if len(s.manifestData) > 0 {
		c.SetManifestData(s.manifestData)
	} else {
		c.SetManifestPath(cfg.String("manifest_path"))
	}
	c.SetCredentials(credentialsFromConfig(cfg))
	if err := c.Validate(); err != nil {
		return fmt.Errorf("httpapi source: validate connector: %w", err)
	}
	if err := c.Configure(ctx); err != nil {
		return fmt.Errorf("httpapi source: configure connector: %w", err)
	}
	s.connector = c
	return nil
}

func (s *Source) Discover(ctx context.Context, _ ingestion.DiscoverOpts) (ingestion.DiscoverResult, error) {
	if s.connector.manifest == nil {
		return ingestion.DiscoverResult{}, fmt.Errorf("httpapi source: discover before configure")
	}
	res, err := s.connector.Discover(ctx, pipeline.DiscoverOptions{Logger: slog.Default()})
	if err != nil {
		return ingestion.DiscoverResult{}, err
	}
	if len(res.Resources) == 0 {
		out := make([]ingestion.Resource, 0, len(s.connector.manifest.Resources))
		for _, r := range s.connector.manifest.Resources {
			schema, _ := s.Schema(ctx, r.Name)
			out = append(out, ingestion.Resource{
				Name:       r.Name,
				Selector:   r.Name,
				Selectable: true,
				PrimaryKey: append([]string(nil), r.PrimaryKey...),
				Schema:     &schema,
			})
		}
		return ingestion.DiscoverResult{Resources: out}, nil
	}
	out := make([]ingestion.Resource, 0, len(res.Resources))
	for _, r := range res.Resources {
		name := r.ID
		if name == "" {
			name = r.Name
		}
		out = append(out, ingestion.Resource{
			Name:        name,
			Selector:    encodeSelector(r.Kind, r.ID),
			Selectable:  true,
			DisplayName: r.Name,
			Metadata:    r.Metadata,
		})
	}
	return ingestion.DiscoverResult{Resources: out}, nil
}

func (s *Source) Extract(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts) error {
	return s.extract(ctx, sink, opts, nil, nil)
}

func (s *Source) ExtractFrom(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts, prev map[string]ingestion.Checkpoint) error {
	resumeCursors := make(map[string]string, len(prev))
	resumeWatermarks := make(map[string]map[string]string, len(prev))
	for resource, cp := range prev {
		ks, ok := checkpoint.ParseKeyset(cp)
		if !ok || len(ks.Shards) == 0 || len(ks.Shards[0].Key) == 0 {
			continue
		}
		key := ks.Shards[0].Key
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

func (s *Source) PlanResources(_ context.Context, resources, selectors []string) ([]string, error) {
	if s.connector.manifest == nil {
		return nil, fmt.Errorf("httpapi source: plan resources before configure")
	}
	return s.planResources(resources, selectors)
}

func (s *Source) PlanResume(_ context.Context, resources []string, prev map[string]ingestion.Checkpoint) (map[string]ingestion.Checkpoint, error) {
	if s.connector.manifest == nil {
		return nil, fmt.Errorf("httpapi source: plan resume before configure")
	}
	resources, err := s.planResources(resources, nil)
	if err != nil {
		return nil, err
	}
	plan := make(map[string]ingestion.Checkpoint, len(resources))
	for _, resource := range resources {
		cols := append([]string{"cursor"}, s.watermarkKeys(resource)...)
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

func (s *Source) Schema(_ context.Context, resource string) (ingestion.RecordSchema, error) {
	if s.connector.manifest == nil {
		return ingestion.RecordSchema{}, fmt.Errorf("httpapi source: schema before configure")
	}
	base := s.baseResourceName(resource)
	for _, res := range s.connector.manifest.Resources {
		if res.Name != base {
			continue
		}
		return ingestion.RecordSchema{Resource: resource, Fields: schemaFields(res), PrimaryKey: append([]string(nil), res.PrimaryKey...)}, nil
	}
	return ingestion.RecordSchema{}, fmt.Errorf("httpapi source: unknown resource %q", resource)
}

func (s *Source) extract(ctx context.Context, sink ingestion.RecordSink, opts ingestion.ExtractOpts, resumeCursors map[string]string, resumeWatermarks map[string]map[string]string) error {
	if s.connector.manifest == nil {
		return fmt.Errorf("httpapi source: extract before configure")
	}

	ch := make(chan pipeline.Record, max(1, opts.Parallelism*2))
	done := make(chan struct{})
	var once sync.Once
	var sinkErr error
	var mu sync.Mutex

	setErr := func(err error) {
		if err == nil {
			return
		}
		mu.Lock()
		if sinkErr == nil {
			sinkErr = err
		}
		mu.Unlock()
		once.Do(func() { close(done) })
	}
	errFn := func() error {
		mu.Lock()
		defer mu.Unlock()
		return sinkErr
	}

	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		for rec := range ch {
			if err := sink.Push(toIngestionRecord(rec)); err != nil {
				setErr(err)
				return
			}
		}
	}()

	err := s.connector.Extract(ctx, pipeline.ExtractOptions{
		Sink:             pipeline.NewRecordSink(ch, done, errFn),
		Logger:           slog.Default(),
		Reporter:         pipeline.NoopReporter{},
		Resources:        s.connectorResources(opts.Resources),
		EnabledResources: enabledResources(opts.Selectors),
		ResumeCursors:    resumeCursors,
		ResumeWatermarks: resumeWatermarks,
	})
	close(ch)
	wg.Wait()
	if err != nil {
		return err
	}
	return errFn()
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

func (s *Source) syntheticParentsFor(res manifest.Resource, refs []pipeline.ResourceRef) []Capture {
	if res.Parent == nil {
		return nil
	}
	parent, ok := s.manifestResource(res.Parent.Resource)
	if !ok {
		return nil
	}
	var out []Capture
	for _, disc := range s.connector.manifest.Discovery {
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

func (s *Source) watermarkKeys(resource string) []string {
	resource = s.baseResourceName(resource)
	for _, res := range s.connector.manifest.Resources {
		if res.Name == resource && res.Incremental != nil {
			return []string{incremental.CheckpointKey(*res.Incremental)}
		}
	}
	return nil
}

func (s *Source) Teardown(ctx context.Context) error {
	return s.connector.Teardown(ctx)
}

func credentialsFromConfig(cfg ingestion.Config) map[string]string {
	creds := map[string]string{}
	for k, v := range cfg.Raw() {
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
		}
	}
	return creds
}

func enabledResources(selectors []string) []pipeline.ResourceRef {
	if len(selectors) == 0 {
		return nil
	}
	out := make([]pipeline.ResourceRef, 0, len(selectors))
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

func decodeSelector(selector string) (pipeline.ResourceRef, bool) {
	var token selectorToken
	if err := json.Unmarshal([]byte(selector), &token); err != nil || token.ID == "" {
		return pipeline.ResourceRef{}, false
	}
	return pipeline.ResourceRef{Kind: token.Kind, ID: token.ID}, true
}

func toIngestionRecord(rec pipeline.Record) ingestion.Record {
	out := ingestion.Record{
		Resource: rec.Resource,
		ID:       recordID(rec.KeyJSON),
		Op:       operationToPkg(rec.Operation),
		Data:     rec.DataJSON,
	}
	if !rec.Projected {
		out.Data = recordData(rec.KeyJSON, rec.DataJSON)
	}
	if rec.Cursor != "" || len(rec.Watermarks) > 0 {
		out.Key = []string{rec.Cursor}
		keys := make([]string, 0, len(rec.Watermarks))
		for key := range rec.Watermarks {
			keys = append(keys, key)
		}
		sort.Strings(keys)
		for _, key := range keys {
			out.Key = append(out.Key, rec.Watermarks[key])
		}
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

func operationToPkg(op pipeline.Operation) ingestion.Operation {
	switch op {
	case pipeline.OperationDelete:
		return ingestion.OpDelete
	case pipeline.OperationUpdate:
		return ingestion.OpUpdate
	default:
		return ingestion.OpInsert
	}
}
