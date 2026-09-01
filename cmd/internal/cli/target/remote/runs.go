package remote

import (
	"context"
	"fmt"
	"time"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

// runListPageSize bounds one history listing; the deployment orders newest
// first.
const runListPageSize = 50

// ListRuns returns the deployment's run history, optionally for one pipeline.
func (t *Target) ListRuns(ctx context.Context, pipeline string) (model.RunList, error) {
	message := &ingestionv1.ListRunsRequest{
		Pagination: &ingestionv1.PaginationRequest{PageSize: runListPageSize},
	}
	pipelines, err := t.client.ListPipelines(ctx, connect.NewRequest(&ingestionv1.ListPipelinesRequest{IncludeVersions: true}))
	if err != nil {
		return model.RunList{}, t.rpcError(err)
	}
	names := make(map[string]string, len(pipelines.Msg.GetPipelines()))
	versions := map[string]string{}
	for _, item := range pipelines.Msg.GetPipelines() {
		names[item.GetId()] = item.GetName()
		if pipeline != "" && item.GetName() == pipeline {
			message.PipelineId = item.GetId()
		}
		for _, version := range append(item.GetVersions(), item.GetCurrentVersion()) {
			if version.GetId() != "" {
				versions[version.GetId()] = "v" + revisionOf(version.GetVersion())
			}
		}
	}
	if pipeline != "" && message.GetPipelineId() == "" {
		return model.RunList{}, fmt.Errorf("pipeline %q does not exist", pipeline)
	}
	response, err := t.client.ListRuns(ctx, connect.NewRequest(message))
	if err != nil {
		return model.RunList{}, t.rpcError(err)
	}
	runs := response.Msg.GetRuns()
	list := model.RunList{Items: make([]model.RunSummary, 0, len(runs))}
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
