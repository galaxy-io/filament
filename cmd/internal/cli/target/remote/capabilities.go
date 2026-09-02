package remote

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// PipelineModes resolves modes through the deployment's authoritative graph
// validator, matching the web canvas rather than inferring policy from the
// connector catalog.
func (t *Target) PipelineModes(ctx context.Context, pipeline model.Pipeline) (model.PipelineModes, error) {
	graph, _, err := t.buildGraph(ctx, pipeline)
	if err != nil {
		return model.PipelineModes{}, err
	}
	response, err := t.client.ValidatePipeline(ctx, connect.NewRequest(&ingestionv1.ValidatePipelineRequest{Graph: graph}))
	if err != nil {
		return model.PipelineModes{}, t.rpcError(err)
	}
	if len(response.Msg.GetEdges()) == 0 {
		for _, validationErr := range response.Msg.GetErrors() {
			if validationErr.GetMessage() != "" {
				return model.PipelineModes{}, fmt.Errorf("validate pipeline: %s", validationErr.GetMessage())
			}
		}
		return model.PipelineModes{}, fmt.Errorf("validate pipeline returned no route capabilities")
	}
	result := model.PipelineModes{}
	readSets := [][]string{}
	for _, edge := range response.Msg.GetEdges() {
		switch edge.GetReplication() {
		case ingestionv1.ReplicationMode_REPLICATION_MODE_CDC:
			result.Replication = "cdc"
			result.ReadModes = []string{"cdc"}
		case ingestionv1.ReplicationMode_REPLICATION_MODE_STANDARD:
			if result.Replication == "" {
				result.Replication = "standard"
			}
		}
		for _, mode := range edge.GetSupportedWriteModes() {
			result.WriteModes = appendUniqueString(result.WriteModes, writeModeString(mode))
		}
		for _, resource := range edge.GetResources() {
			modes := []string{}
			for _, mode := range resource.GetSupportedReadModes() {
				modes = appendUniqueString(modes, readModeString(mode))
			}
			readSets = append(readSets, modes)
		}
	}
	if result.Replication != "cdc" {
		result.ReadModes = intersectStrings(readSets)
		if len(result.ReadModes) == 0 {
			result.ReadModes = []string{"full"}
		}
	}
	return result, nil
}

func appendUniqueString(values []string, value string) []string {
	if value == "" {
		return values
	}
	for _, existing := range values {
		if existing == value {
			return values
		}
	}
	return append(values, value)
}

func intersectStrings(sets [][]string) []string {
	if len(sets) == 0 {
		return nil
	}
	result := append([]string(nil), sets[0]...)
	for _, set := range sets[1:] {
		filtered := result[:0]
		for _, candidate := range result {
			for _, value := range set {
				if candidate == value {
					filtered = append(filtered, candidate)
					break
				}
			}
		}
		result = filtered
	}
	return result
}
