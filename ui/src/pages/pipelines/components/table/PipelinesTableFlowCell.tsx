import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineFlow from "@/pages/pipelines/components/flow/PipelineFlow";
import { usePipelineFlowEndpoints } from "@/pages/pipelines/hooks/usePipelineFlowEndpoints";

interface PipelinesTableFlowCellProps {
  pipeline: Pipeline;
}

const PipelinesTableFlowCell = ({ pipeline }: PipelinesTableFlowCellProps) => {
  const { source, sinks, hasEdges, isLoading } = usePipelineFlowEndpoints(pipeline.id);

  return <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} isLoading={isLoading} />;
};

export default PipelinesTableFlowCell;
