import { create } from "@bufbuild/protobuf";

import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetPipelineVersionRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { mapVersionNodesToFlowEndpoints } from "@/pages/pipelines/components/flow/utils";

import { useListConnectionsQuery } from "@/api/queries/connections";
import { useGetPipelineVersionQuery } from "@/api/queries/pipeline_versions";

export const usePipelineFlowEndpoints = (pipelineId: Pipeline["id"]) => {
  const { data: versionData } = useGetPipelineVersionQuery({
    input: create(GetPipelineVersionRequestSchema, { pipelineId }),
    options: { retry: false },
  });
  const { data: connectionsData } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, {}),
  });

  const { source, sinks } = mapVersionNodesToFlowEndpoints(
    versionData?.version?.nodes ?? [],
    connectionsData?.connections ?? [],
  );

  return { source, sinks, hasEdges: (versionData?.version?.edges ?? []).length > 0 };
};
