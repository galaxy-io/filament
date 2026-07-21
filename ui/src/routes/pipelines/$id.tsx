import { create } from "@bufbuild/protobuf";
import { createFileRoute, Outlet, useParams } from "@tanstack/react-router";

import PipelineLayout from "@/layouts/pipeline/PipelineLayout";
import { PipelineStatus } from "@/layouts/pipeline/types";

import PipelineCanvasProvider from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import { mapPipelineVersionToCanvasState } from "@/pages/pipelines/canvas/utils";

import { useListConnectionsQuery } from "@/api/queries/connectors";
import { useGetPipelineQuery, useGetPipelineVersionQuery } from "@/api/queries/pipelines";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import {
  GetPipelineRequestSchema,
  GetPipelineVersionRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

const PipelineLayoutRoute = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  const { data: pipelineData, isLoading: isLoadingPipeline } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  // version 0 = the pipeline's current version. A pipeline with no saved
  // versions yet returns NotFound, which renders as an empty canvas.
  const { data: versionData, isLoading: isLoadingVersion } = useGetPipelineVersionQuery({
    input: create(GetPipelineVersionRequestSchema, { pipelineId: id }),
    options: { retry: false },
  });
  const { data: connectionsData, isLoading: isLoadingConnections } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, { kind: ConnectorKind.UNSPECIFIED }),
  });

  const pipeline = pipelineData?.pipeline;
  const version = versionData?.version;

  if (isLoadingPipeline || isLoadingVersion || isLoadingConnections || !pipeline) {
    return null;
  }

  return (
    <PipelineCanvasProvider
      initialState={mapPipelineVersionToCanvasState(version, connectionsData?.connections ?? [])}
    >
      <PipelineLayout pipeline={pipeline} currentVersion={version} status={PipelineStatus.DRAFT}>
        <Outlet />
      </PipelineLayout>
    </PipelineCanvasProvider>
  );
};

export const Route = createFileRoute("/pipelines/$id")({
  component: PipelineLayoutRoute,
});
