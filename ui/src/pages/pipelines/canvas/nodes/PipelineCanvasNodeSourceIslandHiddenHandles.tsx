import type { FC } from "react";

import { styled } from "@linaria/react";
import { Position } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import PipelineCanvasNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeHandle";
import type { PipelineCanvasNodeTableInfo } from "@/pages/pipelines/canvas/types";

const HiddenWrapper = styled.div`
  display: none;
`;

interface PipelineCanvasNodeSourceIslandHiddenHandlesProps {
  tables: PipelineCanvasNodeTableInfo[];
}

const PipelineCanvasNodeSourceIslandHiddenHandles: FC<
  PipelineCanvasNodeSourceIslandHiddenHandlesProps
> = ({ tables }) => (
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
