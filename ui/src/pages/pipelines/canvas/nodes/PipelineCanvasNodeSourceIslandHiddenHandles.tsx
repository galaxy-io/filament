import { styled } from "@linaria/react";
import { Position } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import PipelineCanvasNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeHandle";
import type { PipelineCanvasNodeTableInfo } from "@/pages/pipelines/canvas/types";

// Keeps handles mounted for connected tables the list no longer renders, so React Flow can
// still resolve their edges; PipelineCanvasEdge anchors those to the badge.
const HiddenWrapper = styled.div`
  display: none;
`;

interface PipelineCanvasNodeSourceIslandHiddenHandlesProps {
  tables: PipelineCanvasNodeTableInfo[];
}

const PipelineCanvasNodeSourceIslandHiddenHandles = ({
  tables,
}: PipelineCanvasNodeSourceIslandHiddenHandlesProps) => (
  <HiddenWrapper>
    {tables.map((table) => (
      <PipelineCanvasNodeHandle
        key={table.name}
        id={table.name}
        kind={ConnectorKind.SOURCE}
        position={Position.Right}
        isConnected={table.isConnected}
      />
    ))}
  </HiddenWrapper>
);

export default PipelineCanvasNodeSourceIslandHiddenHandles;
