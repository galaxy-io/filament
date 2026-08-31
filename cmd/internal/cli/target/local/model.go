// Package local implements CLI target operations backed by a local YAML
// document and the in-process Filament runtime.
package local

// ConfigVersion is the current local YAML document version.
const ConfigVersion = 1

// Document is the persisted local connection and pipeline configuration.
type Document struct {
	Version   int                   `yaml:"version"`
	Sources   map[string]Connection `yaml:"sources,omitempty"`
	Sinks     map[string]Connection `yaml:"sinks,omitempty"`
	Pipelines map[string]Pipeline   `yaml:"pipelines,omitempty"`
}

// Connection is a locally persisted source or sink configuration.
type Connection struct {
	Type   string         `yaml:"type"`
	Config map[string]any `yaml:"config,omitempty"`
}

// Pipeline is a locally persisted source-to-sink pipeline.
type Pipeline struct {
	Source    PipelineNode `yaml:"source"`
	Sink      PipelineNode `yaml:"sink"`
	Resources []string     `yaml:"resources,omitempty"`
	SyncMode  string       `yaml:"sync_mode"`
	WriteMode string       `yaml:"write_mode"`
}

// PipelineNode references a local connection and supplies pipeline-scoped
// connector configuration.
type PipelineNode struct {
	Ref    string         `yaml:"ref"`
	Config map[string]any `yaml:"config,omitempty"`
}

// NewDocument returns an initialized empty local configuration document.
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
