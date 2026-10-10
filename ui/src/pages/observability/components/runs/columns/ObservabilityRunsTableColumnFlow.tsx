import type { FC } from "react";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import PipelineFlow, { PipelineFlowSize } from "@/components/pipelines/PipelineFlow";

interface ObservabilityRunsTableColumnFlowProps {
  runInfo: RunInfo;
}

const ObservabilityRunsTableColumnFlow: FC<ObservabilityRunsTableColumnFlowProps> = ({
  runInfo,
}) => (
  <PipelineFlow
    sourceId={runInfo.sourceConnectionId}
    sinkIds={runInfo.sinkConnectionId ? [runInfo.sinkConnectionId] : []}
    size={PipelineFlowSize.SMALL}
  />
);

export default ObservabilityRunsTableColumnFlow;
