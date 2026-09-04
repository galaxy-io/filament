package remote

import (
	"context"
	"fmt"
	"slices"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// PipelineModes asks the deployment's graph validator which modes a route
// supports, the same source of truth the web canvas uses.
func (t *Target) PipelineModes(ctx context.Context, pipeline model.Pipeline) (model.PipelineModes, error) {
	graph, _, err := t.graphToProto(ctx, pipeline)
	if err != nil {
		return model.PipelineModes{}, err
	}
	response, err := t.client.ValidatePipeline(ctx, connect.NewRequest(&ingestionv1.ValidatePipelineRequest{Graph: graph}))
	if err != nil {
		return model.PipelineModes{}, t.rpcError(err)
	}
	edges := response.Msg.GetEdges()
	if len(edges) == 0 {
		for _, problem := range response.Msg.GetErrors() {
			if problem.GetMessage() != "" {
				return model.PipelineModes{}, fmt.Errorf("validate pipeline: %s", problem.GetMessage())
			}
		}
		return model.PipelineModes{}, fmt.Errorf("validate pipeline returned no route")
	}
	result := model.PipelineModes{Replication: "standard"}
	var readSets [][]string
	for _, edge := range edges {
		if edge.GetReplication() == ingestionv1.ReplicationMode_REPLICATION_MODE_CDC {
			result.Replication = model.SyncModeCDC
		}
		for _, mode := range edge.GetSupportedWriteModes() {
			result.WriteModes = appendUnique(result.WriteModes, writeModeString(mode))
		}
		for _, resource := range edge.GetResources() {
			var modes []string
			for _, mode := range resource.GetSupportedReadModes() {
				modes = appendUnique(modes, readModeString(mode))
			}
			readSets = append(readSets, modes)
		}
	}
	if result.Replication == model.SyncModeCDC {
		result.ReadModes = []string{model.SyncModeCDC}
		return result, nil
	}
	result.ReadModes = intersect(readSets)
	if len(result.ReadModes) == 0 {
		result.ReadModes = []string{"full"}
	}
	return result, nil
}

func appendUnique(values []string, value string) []string {
	if value == "" || slices.Contains(values, value) {
		return values
	}
	return append(values, value)
}

// intersect keeps the values present in every set, in first-set order.
func intersect(sets [][]string) []string {
	if len(sets) == 0 {
		return nil
	}
	result := slices.Clone(sets[0])
	for _, set := range sets[1:] {
		result = slices.DeleteFunc(result, func(value string) bool { return !slices.Contains(set, value) })
	}
	return result
}
