package remote

import (
	"context"
	"fmt"

	"connectrpc.com/connect"

	ingestionv1 "github.com/galaxy-io/filament/api/ingestion/v1"
	cliapp "github.com/galaxy-io/filament/cmd/internal/cli/app"
	"github.com/galaxy-io/filament/cmd/internal/cli/model"
)

var _ cliapp.RunHistoryTarget = (*Target)(nil)

// ListRuns returns one page of the deployment's run history, optionally for
// one pipeline, with pipeline names and version numbers resolved.
func (t *Target) ListRuns(ctx context.Context, request model.RunListRequest) (model.RunList, error) {
	names, pipelineID, err := t.pipelineNames(ctx, request.Pipeline)
	if err != nil {
		return model.RunList{}, err
	}
	response, err := t.client.ListRuns(ctx, connect.NewRequest(&ingestionv1.ListRunsRequest{
		PipelineId: pipelineID, Pagination: pagination(request.PageRequest),
	}))
	if err != nil {
		return model.RunList{}, t.rpcError(err)
	}
	runs := response.Msg.GetRuns()
	versions, err := t.versionLabels(ctx, runs)
	if err != nil {
		return model.RunList{}, err
	}
	list := model.RunList{Pipeline: request.Pipeline, Items: make([]model.RunSummary, 0, len(runs)), Page: pageInfo(response.Msg.GetPagination())}
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

// pipelineNames maps ids to names, deleted pipelines included since their
// runs are still history, and resolves the selected pipeline's id.
func (t *Target) pipelineNames(ctx context.Context, selected string) (map[string]string, string, error) {
	pipelines, err := drain(func(cursor string) ([]*ingestionv1.Pipeline, *ingestionv1.PaginationResponse, error) {
		response, err := t.client.ListPipelines(ctx, connect.NewRequest(&ingestionv1.ListPipelinesRequest{
			IncludeDeleted: true, Pagination: pagination(model.PageRequest{PageSize: pageSize, Cursor: cursor}),
		}))
		if err != nil {
			return nil, nil, t.rpcError(err)
		}
		return response.Msg.GetPipelines(), response.Msg.GetPagination(), nil
	})
	if err != nil {
		return nil, "", err
	}
	names := make(map[string]string, len(pipelines))
	selectedID := ""
	for _, pipeline := range pipelines {
		names[pipeline.GetId()] = pipeline.GetName()
		if pipeline.GetName() == selected {
			selectedID = pipeline.GetId()
		}
	}
	if selected != "" && selectedID == "" {
		return nil, "", fmt.Errorf("pipeline %q does not exist", selected)
	}
	return names, selectedID, nil
}

// versionLabels resolves the version numbers this page of runs refers to,
// one listing per pipeline on the page.
func (t *Target) versionLabels(ctx context.Context, runs []*ingestionv1.RunInfo) (map[string]string, error) {
	wanted := map[string]map[string]bool{}
	for _, run := range runs {
		if run.GetPipelineId() == "" || run.GetPipelineVersionId() == "" {
			continue
		}
		if wanted[run.GetPipelineId()] == nil {
			wanted[run.GetPipelineId()] = map[string]bool{}
		}
		wanted[run.GetPipelineId()][run.GetPipelineVersionId()] = true
	}
	labels := map[string]string{}
	for pipelineID, versionIDs := range wanted {
		versions, err := drain(func(cursor string) ([]*ingestionv1.PipelineVersion, *ingestionv1.PaginationResponse, error) {
			response, err := t.client.ListPipelineVersions(ctx, connect.NewRequest(&ingestionv1.ListPipelineVersionsRequest{
				PipelineId: pipelineID, Pagination: pagination(model.PageRequest{PageSize: pageSize, Cursor: cursor}),
			}))
			if err != nil {
				return nil, nil, t.rpcError(err)
			}
			return response.Msg.GetVersions(), response.Msg.GetPagination(), nil
		})
		if err != nil {
			return nil, err
		}
		for _, version := range versions {
			if versionIDs[version.GetId()] {
				labels[version.GetId()] = "v" + revisionOf(version.GetVersion())
			}
		}
	}
	return labels, nil
}
