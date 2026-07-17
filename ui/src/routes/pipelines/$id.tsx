import { create } from "@bufbuild/protobuf";
import { createFileRoute, Outlet, useParams } from "@tanstack/react-router";

import PipelineLayout from "@/layouts/pipeline/PipelineLayout";
import { PipelineStatus } from "@/layouts/pipeline/types";

import PipelineCanvasProvider from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import { mapPipelineToCanvasState } from "@/pages/pipelines/canvas/utils";

import { useListConnectionsQuery } from "@/api/queries/connectors";
import { useGetPipelineQuery } from "@/api/queries/pipelines";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetPipelineRequestSchema } from "@/gen/ingestion/v1/pipelines_pb";

const PipelineLayoutRoute = () => {
  const { id } = useParams({ from: "/pipelines/$id" });

  const { data: pipelineData, isLoading: isLoadingPipeline } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id }),
  });
  const { data: connectionsData, isLoading: isLoadingConnections } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, { kind: ConnectorKind.UNSPECIFIED }),
  });

  const pipeline = pipelineData?.pipeline;

  if (isLoadingPipeline || isLoadingConnections || !pipeline) {
    return null;
  }

  return (
    <PipelineCanvasProvider
      initialState={mapPipelineToCanvasState(pipeline, connectionsData?.connections ?? [])}
    >
      <PipelineLayout pipeline={pipeline} status={PipelineStatus.DRAFT}>
        <Outlet />
      </PipelineLayout>
    </PipelineCanvasProvider>
  );
};

export const Route = createFileRoute("/pipelines/$id")({
  component: PipelineLayoutRoute,
});
