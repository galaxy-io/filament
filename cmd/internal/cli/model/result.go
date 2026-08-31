// Package model defines target-neutral values exchanged by CLI use cases,
// target adapters, and presentation adapters.
package model

// ConnectionList is the result of listing saved connections of one kind.
type ConnectionList struct {
	Kind  string
	Items []ConnectionSummary
}

// ConnectionSummary is the presentation-neutral subset of a connection used
// by list surfaces.
type ConnectionSummary struct {
	Name        string
	Connector   string
	Description string
}

// PipelineList is the result of listing saved pipelines.
type PipelineList struct {
	Items []PipelineSummary
}

// PipelineSummary is the presentation-neutral subset of a pipeline used by
// list surfaces.
type PipelineSummary struct {
	Name          string
	Source        string
	Sink          string
	ResourceCount int
	AllResources  bool
	SyncMode      string
	WriteMode     string
}

// ResourceList is the result of discovering resources for a source.
type ResourceList struct {
	Source string
	Items  []ResourceSummary
}

// ResourceSummary describes one discovered source resource.
type ResourceSummary struct {
	Name          string
	DisplayName   string
	Selectable    bool
	PrimaryKey    []string
	EstimatedRows int64
}
