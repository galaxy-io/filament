import { memo } from "react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import type { PipelineNodeSinkProps } from "@/pages/pipelines/canvas/nodes/types";
import PipelineNode from "@/pages/pipelines/canvas/nodes/PipelineNode";

const PipelineNodeSink = memo(({ data, selected }: PipelineNodeSinkProps) => {
  return (
    <PipelineNode
      connector={data.label}
      kind={ConnectorKind.SINK}
      handleId="input"
      isConnected={true}
      isSelected={selected}
    />
  );
});

PipelineNodeSink.displayName = "PipelineNodeSink";

export default PipelineNodeSink;
