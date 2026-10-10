import type { FC } from "react";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineFlow from "@/components/pipelines/PipelineFlow";
import { mapVersionNodesToFlowEndpoints } from "@/components/pipelines/utils";

interface PipelinesTableColumnFlowProps {
  pipeline: Pipeline;
}

const PipelinesTableColumnFlow: FC<PipelinesTableColumnFlowProps> = ({ pipeline }) => {
  const graph = pipeline.currentVersion?.graph;
  const { sourceId, sinkIds } = mapVersionNodesToFlowEndpoints(graph?.nodes ?? []);

  return (
    <PipelineFlow
      sourceId={sourceId}
      sinkIds={sinkIds}
      hasEdges={(graph?.edges ?? []).length > 0}
    />
  );
};

export default PipelinesTableColumnFlow;
