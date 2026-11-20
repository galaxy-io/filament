package compile

import (
	"context"
	"fmt"

	"github.com/galaxy-io/filament"
	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
)

// continuousRouteSuffix marks the stream of a source that splits execution.
// Its bounded runs keep the plain route key, so the stream and the runs hold
// separate active-run slots and checkpoints on the same source and sink.
const continuousRouteSuffix = "/continuous"

// routeKey names the route between a source node and a sink node.
func routeKey(from, to string) string { return fmt.Sprintf("route/%s/%s", from, to) }

// ContinuousRouteKey is the stream route key of a route whose source has
// spec. A source that splits execution suffixes it.
func ContinuousRouteKey(spec filament.ConnectorSpec, route string) string {
	if spec.Webhook.SplitsExecution() {
		return route + continuousRouteSuffix
	}
	return route
}

// executionEdges keeps the edges that run in execution. A source that splits
// execution decides it per resource, so a pipeline with both kinds compiles
// its API resources into bounded runs and its webhook resources into a stream.
// Edges of every other source run in whatever mode the start asked for.
func (c *Compiler) executionEdges(ctx context.Context, edges []*ingestionv1.PipelineEdge, nodes map[string]*ingestionv1.PipelineNode, connections map[string]filament.Connection, execution filament.ExecutionMode) ([]*ingestionv1.PipelineEdge, error) {
	specs := map[string]filament.ConnectorSpec{}
	kept := make([]*ingestionv1.PipelineEdge, 0, len(edges))
	for _, edge := range edges {
		node := nodes[edge.GetFromNode()]
		if node == nil {
			// groupEdges reports the dangling reference.
			kept = append(kept, edge)
			continue
		}
		connector := connections[node.GetConnectionId()].Connector
		spec, ok := specs[connector]
		if !ok {
			var err error
			if spec, err = c.Worker.SourceSpec(ctx, connector); err != nil {
				return nil, err
			}
			specs[connector] = spec
		}
		if filament.ResourceExecution(spec, edge.GetResource(), execution) == execution {
			kept = append(kept, edge)
		}
	}
	if len(kept) == 0 && len(edges) > 0 {
		return nil, fmt.Errorf("%w: pipeline has no resources that run %s", ErrInvalid, execution)
	}
	return kept, nil
}

// boundedGroups groups the edges that run bounded into routes and checks
// resync against them. It returns the kept edges for subsetting.
func (c *Compiler) boundedGroups(ctx context.Context, edges []*ingestionv1.PipelineEdge, nodes map[string]*ingestionv1.PipelineNode, connections map[string]filament.Connection, resync []string) ([]*routeGroup, []*ingestionv1.PipelineEdge, error) {
	edges, err := c.executionEdges(ctx, edges, nodes, connections, filament.ExecutionBounded)
	if err != nil {
		return nil, nil, err
	}
	groups, err := groupEdges(edges, nodes)
	if err != nil {
		return nil, nil, err
	}
	if err := validateResync(groups, resync); err != nil {
		return nil, nil, err
	}
	return groups, edges, nil
}
