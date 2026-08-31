// Package app contains target-neutral CLI use cases.
package app

import (
	"context"

	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// QueryTarget supplies read operations for one selected local or remote target.
type QueryTarget interface {
	ListConnections(context.Context, string) (model.ConnectionList, error)
	ListPipelines(context.Context) (model.PipelineList, error)
}

// Queries exposes target-neutral read use cases to CLI input adapters.
type Queries struct {
	target QueryTarget
}

// NewQueries binds read use cases to the selected target.
func NewQueries(target QueryTarget) *Queries {
	return &Queries{target: target}
}

// Connections lists connections of one kind from the selected target.
func (q *Queries) Connections(ctx context.Context, kind string) (model.ConnectionList, error) {
	return q.target.ListConnections(ctx, kind)
}

// Pipelines lists pipelines from the selected target.
func (q *Queries) Pipelines(ctx context.Context) (model.PipelineList, error) {
	return q.target.ListPipelines(ctx)
}
