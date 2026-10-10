import { create } from "@bufbuild/protobuf";

import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { mapVersionNodesToFlowEndpoints } from "@/components/pipelines/utils";

import { useListConnectionsQuery } from "@/api/queries/connections";

export const usePipelineFlowEndpoints = (pipeline: Pipeline) => {
  const { data: connectionsData, isLoading } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, {}),
  });
  const graph = pipeline.currentVersion?.graph;

  const { source, sinks } = mapVersionNodesToFlowEndpoints(
    graph?.nodes ?? [],
    connectionsData?.connections ?? [],
  );

  return { source, sinks, hasEdges: (graph?.edges ?? []).length > 0, isLoading };
};
