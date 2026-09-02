package remote

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

const (
	defaultRunListPageSize = 25
)

// ListRuns returns the deployment's run history, optionally for one pipeline.
func (t *Target) ListRuns(ctx context.Context, request model.RunListRequest) (model.RunList, error) {
	pageSize := request.PageSize
	if pageSize <= 0 {
		pageSize = defaultRunListPageSize
	}
	names, pipelineID, err := t.runPipelineNames(ctx, request.Pipeline)
	if err != nil {
		return model.RunList{}, err
	}
	message := &ingestionv1.ListRunsRequest{
		PipelineId: pipelineID,
		Pagination: paginationRequest(pageSize, request.Cursor),
	}
	response, err := t.client.ListRuns(ctx, connect.NewRequest(message))
	if err != nil {
		return model.RunList{}, t.rpcError(err)
	}
	runs := response.Msg.GetRuns()
	versionIDs := make(map[string]map[string]struct{})
	for _, run := range runs {
		if run.GetPipelineId() == "" || run.GetPipelineVersionId() == "" {
			continue
		}
		if versionIDs[run.GetPipelineId()] == nil {
			versionIDs[run.GetPipelineId()] = map[string]struct{}{}
		}
		versionIDs[run.GetPipelineId()][run.GetPipelineVersionId()] = struct{}{}
	}
	versions := make(map[string]string)
	for id, wanted := range versionIDs {
		labels, err := t.runVersionLabels(ctx, id, wanted)
		if err != nil {
			return model.RunList{}, err
		}
		for versionID, label := range labels {
			versions[versionID] = label
		}
	}
	pagination := response.Msg.GetPagination()
	list := model.RunList{
		Items: make([]model.RunSummary, 0, len(runs)), Total: int(pagination.GetTotal()), Pipeline: request.Pipeline,
		NextCursor: pagination.GetNextCursor(), PreviousCursor: pagination.GetPreviousCursor(),
	}
	for _, run := range runs {
		name := names[run.GetPipelineId()]
		if name == "" {
			name = run.GetPipelineId()
		}
		list.Items = append(list.Items, model.RunSummary{
			ID:        run.GetId(),
			Pipeline:  name,
			Version:   versions[run.GetPipelineVersionId()],
			Status:    runStatusString(run.GetStatus()),
			Records:   run.GetRecords(),
			Bytes:     run.GetBytes(),
			StartedAt: timeFromMillis(run.GetStartedAt()),
			EndedAt:   timeFromMillis(run.GetEndedAt()),
			Error:     run.GetError(),
		})
	}
	return list, nil
}

// runPipelineNames resolves names without requesting every graph version. It
// includes deleted pipelines because their historical runs remain useful.
func (t *Target) runPipelineNames(ctx context.Context, selected string) (map[string]string, string, error) {
	names := map[string]string{}
	selectedID := ""
	cursor := ""
	for {
		response, err := t.client.ListPipelines(ctx, connect.NewRequest(&ingestionv1.ListPipelinesRequest{
			IncludeDeleted: true,
			Pagination:     paginationRequest(internalPageSize, cursor),
		}))
		if err != nil {
			return nil, "", t.rpcError(err)
		}
		for _, pipeline := range response.Msg.GetPipelines() {
			names[pipeline.GetId()] = pipeline.GetName()
			if pipeline.GetName() == selected {
				selectedID = pipeline.GetId()
			}
		}
		cursor = response.Msg.GetPagination().GetNextCursor()
		if cursor == "" {
			break
		}
	}
	if selected != "" && selectedID == "" {
		return nil, "", fmt.Errorf("pipeline %q does not exist", selected)
	}
	return names, selectedID, nil
}

// runVersionLabels fetches only enough version pages to label this result
// page, instead of embedding every version of every pipeline up front.
func (t *Target) runVersionLabels(ctx context.Context, pipelineID string, wanted map[string]struct{}) (map[string]string, error) {
	labels := map[string]string{}
	cursor := ""
	for len(labels) < len(wanted) {
		response, err := t.client.ListPipelineVersions(ctx, connect.NewRequest(&ingestionv1.ListPipelineVersionsRequest{
			PipelineId: pipelineID,
			Pagination: paginationRequest(internalPageSize, cursor),
		}))
		if err != nil {
			return nil, t.rpcError(err)
		}
		for _, version := range response.Msg.GetVersions() {
			if _, ok := wanted[version.GetId()]; ok {
				labels[version.GetId()] = "v" + revisionOf(version.GetVersion())
			}
		}
		cursor = response.Msg.GetPagination().GetNextCursor()
		if cursor == "" {
			break
		}
	}
	return labels, nil
}

func runStatusString(status ingestionv1.RunStatus) string {
	switch status {
	case ingestionv1.RunStatus_RUN_STATUS_REQUESTED:
		return "requested"
	case ingestionv1.RunStatus_RUN_STATUS_SCHEDULED:
		return "scheduled"
	case ingestionv1.RunStatus_RUN_STATUS_RUNNING:
		return "running"
	case ingestionv1.RunStatus_RUN_STATUS_COMPLETED:
		return "completed"
	case ingestionv1.RunStatus_RUN_STATUS_FAILED:
		return "failed"
	case ingestionv1.RunStatus_RUN_STATUS_CANCELED:
		return "canceled"
	case ingestionv1.RunStatus_RUN_STATUS_PAUSED:
		return "paused"
	case ingestionv1.RunStatus_RUN_STATUS_PARTIAL:
		return "partial"
	case ingestionv1.RunStatus_RUN_STATUS_UNSPECIFIED:
		return ""
	default:
		return ""
	}
}

func timeFromMillis(millis int64) time.Time {
	if millis == 0 {
		return time.Time{}
	}
	return time.UnixMilli(millis)
}
