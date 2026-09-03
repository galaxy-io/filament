package model

import (
	"reflect"
	"slices"
	"time"
)

// ConfigVersion is the current configuration document version.
const ConfigVersion = 1

// Document is the target-neutral connection and pipeline configuration.
type Document struct {
	Version   int                   `json:"version" yaml:"version"`
	Sources   map[string]Connection `json:"sources,omitempty" yaml:"sources,omitempty"`
	Sinks     map[string]Connection `json:"sinks,omitempty" yaml:"sinks,omitempty"`
	Pipelines map[string]Pipeline   `json:"pipelines,omitempty" yaml:"pipelines,omitempty"`
}

// EntityMetadata carries target-owned identity and optimistic concurrency
// information. It is intentionally excluded from configuration serialization:
// local targets derive it from the YAML store and remote targets can map it to
// their native IDs and versions.
type EntityMetadata struct {
	ID       string `json:"-" yaml:"-"`
	Revision string `json:"-" yaml:"-"`
}

// Connection is a persisted source or sink configuration.
type Connection struct {
	Metadata   EntityMetadata    `json:"-" yaml:"-"`
	Info       ConnectionInfo    `json:"-" yaml:"-"`
	Type       string            `json:"type" yaml:"type"`
	Config     map[string]any    `json:"config,omitempty" yaml:"config,omitempty"`
	SecretRefs map[string]string `json:"-" yaml:"-"`
}

// ConnectionInfo is target-owned metadata. Local documents leave it zero;
// deployments fill it.
type ConnectionInfo struct {
	Replication string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// NamedConnection identifies a connection returned by a target query.
type NamedConnection struct {
	Kind       string
	Name       string
	Connection Connection
}

// Pipeline is a persisted source-to-sink pipeline.
type Pipeline struct {
	Metadata  EntityMetadata `json:"-" yaml:"-"`
	Info      PipelineInfo   `json:"-" yaml:"-"`
	Source    PipelineNode   `json:"source" yaml:"source"`
	Sink      PipelineNode   `json:"sink" yaml:"sink"`
	Resources []string       `json:"resources,omitempty" yaml:"resources,omitempty"`
	SyncMode  string         `json:"sync_mode" yaml:"sync_mode"`
	WriteMode string         `json:"write_mode" yaml:"write_mode"`
	// Graph retains the target's complete graph. It never reaches YAML, which
	// stays a simple source-to-sink pipeline.
	Graph *PipelineGraph `json:"-" yaml:"-"`
}

// PipelineInfo is target-owned metadata. Local documents leave it zero;
// deployments fill it.
type PipelineInfo struct {
	Schedule      string
	LastRunStatus string
	LastRunAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// EditBlockedReason is set when the deployed graph cannot be represented
	// by the simple source-to-sink editor without losing something.
	EditBlockedReason string
}

// NamedPipeline identifies a pipeline returned by a target query.
type NamedPipeline struct {
	Name     string
	Pipeline Pipeline
}

// PipelineNode references a connection and supplies pipeline-scoped
// connector configuration.
type PipelineNode struct {
	Ref        string            `json:"ref" yaml:"ref"`
	Config     map[string]any    `json:"config,omitempty" yaml:"config,omitempty"`
	SecretRefs map[string]string `json:"-" yaml:"-"`
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

// PipelinesEquivalent compares what a document can express: refs, modes,
// resources, and node configs. Metadata, Info, and Graph are target-owned.
func PipelinesEquivalent(a, b Pipeline) bool {
	return a.Source.Ref == b.Source.Ref && a.Sink.Ref == b.Sink.Ref &&
		a.SyncMode == b.SyncMode && a.WriteMode == b.WriteMode &&
		slices.Equal(a.Resources, b.Resources) &&
		ConfigsEquivalent(a.Source.Config, b.Source.Config) &&
		ConfigsEquivalent(a.Sink.Config, b.Sink.Config)
}

// ConfigsEquivalent treats nil and empty config maps as equal.
func ConfigsEquivalent(a, b map[string]any) bool {
	if len(a) == 0 && len(b) == 0 {
		return true
	}
	return reflect.DeepEqual(a, b)
}
