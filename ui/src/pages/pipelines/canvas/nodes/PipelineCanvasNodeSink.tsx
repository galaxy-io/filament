import { type FC, memo } from "react";

import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { usePipelineCanvasIsConnectionRunning } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasIsConnectionRunning";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasNode from "@/pages/pipelines/canvas/nodes/PipelineCanvasNode";
import type { PipelineCanvasNodeSinkProps } from "@/pages/pipelines/canvas/nodes/types";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

import { createGetConnectionInput, useGetConnectionQuery } from "@/api/queries/connections";

const PipelineCanvasNodeSink: FC<PipelineCanvasNodeSinkProps> = ({ id, data, selected }) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "target" });
  const { removeNode } = usePipelineCanvasActions();
  const { selectNode } = usePipelineCanvasSelection();
  const { data: connectionData } = useGetConnectionQuery({
    input: createGetConnectionInput(data.connectionId),
  });
  const connection = connectionData?.connection;
  const isRunning = usePipelineCanvasIsConnectionRunning(data.connectionId);

  return (
    <PipelineCanvasNode
      connector={connection?.connector ?? ""}
      label={connection?.name ?? data.connectionId}
      kind={ConnectorKind.SINK}
      isConnected={connections.length > 0}
      isSelected={selected}
      isRunning={isRunning}
      onSettings={() => selectNode(id)}
      onDelete={isReadOnly ? undefined : () => removeNode(id)}
    />
  );
};

export default memo(PipelineCanvasNodeSink);
