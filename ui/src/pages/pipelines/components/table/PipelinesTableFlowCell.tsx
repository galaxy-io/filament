import { create } from "@bufbuild/protobuf";

import { ListConnectionsRequestSchema } from "@/gen/ingestion/v1/connections_pb";
import { GetPipelineVersionRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineFlow from "@/pages/pipelines/components/flow/PipelineFlow";
import { mapVersionNodesToFlowEndpoints } from "@/pages/pipelines/components/flow/utils";

import { useListConnectionsQuery } from "@/api/queries/connections";
import { useGetPipelineVersionQuery } from "@/api/queries/pipeline_versions";

interface PipelinesTableFlowCellProps {
  pipeline: Pipeline;
}

const PipelinesTableFlowCell = ({ pipeline }: PipelinesTableFlowCellProps) => {
  const { data: versionData } = useGetPipelineVersionQuery({
    input: create(GetPipelineVersionRequestSchema, { pipelineId: pipeline.id }),
    options: { retry: false },
  });
  const nodes = versionData?.version?.nodes ?? [];
  const hasEdges = (versionData?.version?.edges ?? []).length > 0;

  const { data: connectionsData } = useListConnectionsQuery({
    input: create(ListConnectionsRequestSchema, {}),
  });
  const { source, sinks } = mapVersionNodesToFlowEndpoints(
    nodes,
    connectionsData?.connections ?? [],
  );

  return <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} />;
};

export default PipelinesTableFlowCell;
