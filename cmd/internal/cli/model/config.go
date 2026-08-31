package model

// ConfigVersion is the current configuration document version.
const ConfigVersion = 1

// Document is the target-neutral connection and pipeline configuration.
type Document struct {
	Version   int                   `json:"version" yaml:"version"`
	Sources   map[string]Connection `json:"sources,omitempty" yaml:"sources,omitempty"`
	Sinks     map[string]Connection `json:"sinks,omitempty" yaml:"sinks,omitempty"`
	Pipelines map[string]Pipeline   `json:"pipelines,omitempty" yaml:"pipelines,omitempty"`
}

// Connection is a persisted source or sink configuration.
type Connection struct {
	Type   string         `json:"type" yaml:"type"`
	Config map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
}

// Pipeline is a persisted source-to-sink pipeline.
type Pipeline struct {
	Source    PipelineNode `json:"source" yaml:"source"`
	Sink      PipelineNode `json:"sink" yaml:"sink"`
	Resources []string     `json:"resources,omitempty" yaml:"resources,omitempty"`
	SyncMode  string       `json:"sync_mode" yaml:"sync_mode"`
	WriteMode string       `json:"write_mode" yaml:"write_mode"`
}

// PipelineNode references a connection and supplies pipeline-scoped
// connector configuration.
type PipelineNode struct {
	Ref    string         `json:"ref" yaml:"ref"`
	Config map[string]any `json:"config,omitempty" yaml:"config,omitempty"`
}

// NewDocument returns an initialized empty configuration document.
func NewDocument() Document {
	return Document{
		Version:   ConfigVersion,
		Sources:   map[string]Connection{},
		Sinks:     map[string]Connection{},
		Pipelines: map[string]Pipeline{},
	}
}

// Normalize initializes omitted document collections.
func (d *Document) Normalize() {
	if d.Sources == nil {
		d.Sources = map[string]Connection{}
	}
	if d.Sinks == nil {
		d.Sinks = map[string]Connection{}
	}
	if d.Pipelines == nil {
		d.Pipelines = map[string]Pipeline{}
	}
}
