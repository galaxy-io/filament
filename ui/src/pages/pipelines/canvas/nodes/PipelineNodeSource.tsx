import { memo } from "react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import type { PipelineNodeSourceProps } from "@/pages/pipelines/canvas/nodes/types";
import PipelineNode from "@/pages/pipelines/canvas/nodes/PipelineNode";
import PipelineNodeSourceIsland from "@/pages/pipelines/canvas/nodes/PipelineNodeSourceIsland";

const PipelineNodeSource = memo(({ data, selected }: PipelineNodeSourceProps) => {
  return (
    <PipelineNode
      connector={data.label}
      kind={ConnectorKind.SOURCE}
      handleId="output"
      isConnected={false}
      isSelected={selected}
    >
      {data.tables && data.tables.length > 0 && (
        <PipelineNodeSourceIsland tables={data.tables} isSelected={selected} />
      )}
    </PipelineNode>
  );
});

PipelineNodeSource.displayName = "PipelineNodeSource";

export default PipelineNodeSource;
