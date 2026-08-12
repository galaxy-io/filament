import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineFlow from "@/pages/pipelines/components/flow/PipelineFlow";
import { usePipelineFlowEndpoints } from "@/pages/pipelines/hooks/usePipelineFlowEndpoints";

interface PipelinesTableFlowCellProps {
  pipeline: Pipeline;
}

const PipelinesTableFlowCell = ({ pipeline }: PipelinesTableFlowCellProps) => {
  const { source, sinks, hasEdges } = usePipelineFlowEndpoints(pipeline.id);

  return <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} />;
};

export default PipelinesTableFlowCell;
