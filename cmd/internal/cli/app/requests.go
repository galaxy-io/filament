package app

// ConfigPatch describes target-neutral configuration changes. Values use
// schema field names, never renderer-specific flag or form names.
type ConfigPatch struct {
	Values map[string]any
	Unset  []string
}

// SaveConnectionRequest describes a create or update operation.
type SaveConnectionRequest struct {
	Create    bool
	Kind      string
	Name      string
	Connector string
	Config    ConfigPatch
}

// SavePipelineRequest describes a create or update operation. Empty scalar
// values preserve existing values during an update; Resources uses a pointer
// so an explicitly empty selection can mean all resources.
type SavePipelineRequest struct {
	Create       bool
	Name         string
	Source       string
	Sink         string
	Resources    *[]string
	SyncMode     string
	WriteMode    string
	SourceConfig ConfigPatch
	SinkConfig   ConfigPatch
}

// DiscoverSourceRequest describes discovery from either a saved source or an
// inline connector configuration.
type DiscoverSourceRequest struct {
	Source    string
	Connector string
	Config    ConfigPatch
	Refresh   bool
}

// InlineConnector supplies one side of an inline run.
type InlineConnector struct {
	Connector string
	Config    map[string]any
}

// InlineRun describes a source-to-sink run that is not saved as a pipeline.
type InlineRun struct {
	Source    InlineConnector
	Sink      InlineConnector
	Resources []string
	SyncMode  string
	WriteMode string
}

// RunRequest selects either a saved pipeline with typed overrides or an
// inline transfer.
type RunRequest struct {
	Pipeline  string
	Overrides SavePipelineRequest
	Inline    *InlineRun
}
