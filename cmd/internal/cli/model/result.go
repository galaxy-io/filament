// Package model defines target-neutral values exchanged by CLI use cases,
// target adapters, and presentation adapters.
package model

// DiscoverRequest describes target-side resource discovery.
type DiscoverRequest struct {
	Connector string
	Source    string
	Config    map[string]any
	Refresh   bool
}

// RunResult summarizes a completed run.
type RunResult struct {
	Records int64
	Bytes   int64
}

// RunEvent is a target-neutral resource progress update.
type RunEvent struct {
	Resource string
	Status   string
	Records  int64
	Bytes    int64
	Final    bool
	Error    string
}

// ContextList is the result of listing configured CLI contexts.
type ContextList struct {
	Items []ContextSummary
}

// ContextSummary describes one secret-free local or remote CLI context.
type ContextSummary struct {
	Name     string
	Current  bool
	Kind     string
	Location string
	Tenant   string
}

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
