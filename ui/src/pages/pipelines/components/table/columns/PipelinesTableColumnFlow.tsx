import type { FC } from "react";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { usePipelineFlowEndpoints } from "@/components/pipelines/hooks/usePipelineFlowEndpoints";
import PipelineFlow from "@/components/pipelines/PipelineFlow";

interface PipelinesTableColumnFlowProps {
  pipeline: Pipeline;
}

const PipelinesTableColumnFlow: FC<PipelinesTableColumnFlowProps> = ({ pipeline }) => {
  const { source, sinks, hasEdges, isLoading } = usePipelineFlowEndpoints(pipeline);

  return <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} isLoading={isLoading} />;
};

export default PipelinesTableColumnFlow;
