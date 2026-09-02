package model

import "time"

// ConfigVersion is the current configuration document version.
const ConfigVersion = 1

// AdhocPrefix marks run-scoped entities — inline and override runs — pushed
// to an ephemeral deployment. They exist for one run and must never reach the
// YAML document.
const AdhocPrefix = "adhoc:"

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

// ConnectionInfo carries target-owned descriptive metadata. Local targets
// leave it zero; remote targets fill it from the deployment.
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
	// Graph retains the target's complete graph representation. It is excluded
	// from the local YAML shape, which intentionally remains a simple
	// source-to-sink pipeline.
	Graph *PipelineGraph `json:"-" yaml:"-"`
}

// PipelineInfo carries target-owned descriptive metadata. Local targets
// leave it zero; remote targets fill it from the deployment.
type PipelineInfo struct {
	Schedule      string
	LastRunStatus string
	LastRunAt     time.Time
	CreatedAt     time.Time
	UpdatedAt     time.Time
	// EditBlockedReason is non-empty when the target graph cannot be represented
	// losslessly by the CLI's simple pipeline editor.
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

// PipelineGraph is the lossless target-neutral form of a deployed graph.
// Simple local YAML pipelines are projected into this form only at target
// boundaries.
type PipelineGraph struct {
	Nodes []PipelineGraphNode
	Edges []PipelineGraphEdge
}

// PipelineGraphNode references one saved connection from a graph.
type PipelineGraphNode struct {
	ID         string
	Kind       string
	Connection string
	Config     map[string]any
	SecretRefs map[string]string
}

// PipelineGraphEdge carries every routing setting the deployed API exposes.
type PipelineGraphEdge struct {
	From      string
	To        string
	Resource  string
	Selector  string
	Cursors   []ResourceCursor
	ReadMode  string
	WriteMode string
}

// ResourceCursor is one durable incremental cursor override.
type ResourceCursor struct {
	Resource        string
	Field           string
	LookbackSeconds int64
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
