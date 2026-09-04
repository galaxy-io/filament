// Package model defines target-neutral values exchanged by CLI use cases,
// target adapters, and presentation adapters.
package model

import (
	"time"

	"github.com/galaxy-io/filament"
)

// DiscoverRequest describes target-side resource discovery.
type DiscoverRequest struct {
	Connector string
	Source    string
	Config    map[string]any
	Refresh   bool
}

// RunSubmission is a prepared request ready for target-side submission. A
// saved pipeline carries target identity; an inline run has a nil Pipeline.
type RunSubmission struct {
	Pipeline *EntityReference
	Spec     filament.RunSpec
	Override bool
}

// EntityReference identifies a named target-owned entity.
type EntityReference struct {
	Name     string
	Metadata EntityMetadata
}

// RunRef identifies one target-side run. Route distinguishes the pipeline
// edge when one submission fans out into multiple runs.
type RunRef struct {
	ID    string
	Route string
}

// RunGroup is the result of submitting work to a target.
type RunGroup struct {
	Runs []RunRef
}

// RunResult summarizes all runs produced by one submission.
type RunResult struct {
	Runs    []RunRef
	Status  string
	Records int64
	Bytes   int64
}

// RunEvent is a target-neutral resource progress update.
type RunEvent struct {
	Run      string
	Route    string
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
}

// ConnectionList is the result of listing saved connections of one kind.
type ConnectionList struct {
	Kind  string
	Items []ConnectionSummary
	Page  PageInfo
}

// ConnectionSummary is the presentation-neutral subset of a connection used
// by list surfaces.
type ConnectionSummary struct {
	Name        string
	Connector   string
	Description string
	Replication string
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

// PipelineList is the result of listing saved pipelines.
type PipelineList struct {
	Items []PipelineSummary
	Page  PageInfo
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
	Schedule      string
	LastRunStatus string
	LastRunAt     time.Time
	UpdatedAt     time.Time
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

// PipelineModes is the target's authoritative read and write levers for one
// proposed source-to-sink route.
type PipelineModes struct {
	Replication string
	ReadModes   []string
	WriteModes  []string
}

// PageRequest selects one cursor page. A zero PageSize means the target's
// default; an empty Cursor means the first page.
type PageRequest struct {
	PageSize int32
	Cursor   string
}

// PageInfo is a page's position in its collection. Total is zero when the
// target does not count.
type PageInfo struct {
	Total          int
	NextCursor     string
	PreviousCursor string
}

// Page is one collection page and its position.
type Page[T any] struct {
	Items []T
	PageInfo
}

// RunListRequest selects one page of run history, optionally for a pipeline.
type RunListRequest struct {
	Pipeline string
	PageRequest
}

// RunList is one page of run history.
type RunList struct {
	Pipeline string
	Items    []RunSummary
	Page     PageInfo
}

// RunSummary describes one historical run.
type RunSummary struct {
	ID        string
	Pipeline  string
	Version   string
	Status    string
	Records   int64
	Bytes     int64
	StartedAt time.Time
	EndedAt   time.Time
	Error     string
}
