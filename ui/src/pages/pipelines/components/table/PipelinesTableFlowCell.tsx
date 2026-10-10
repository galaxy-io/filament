import type { FC } from "react";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { usePipelineFlowEndpoints } from "@/components/pipelines/hooks/usePipelineFlowEndpoints";
import PipelineFlow from "@/components/pipelines/PipelineFlow";

interface PipelinesTableFlowCellProps {
  pipeline: Pipeline;
}

const PipelinesTableFlowCell: FC<PipelinesTableFlowCellProps> = ({ pipeline }) => {
  const { source, sinks, hasEdges, isLoading } = usePipelineFlowEndpoints(pipeline.id);

  return <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} isLoading={isLoading} />;
};

export default PipelinesTableFlowCell;
