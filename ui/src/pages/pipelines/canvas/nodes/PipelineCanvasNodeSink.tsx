import { memo } from "react";

import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasNode from "@/pages/pipelines/canvas/nodes/PipelineCanvasNode";
import type { PipelineCanvasNodeSinkProps } from "@/pages/pipelines/canvas/nodes/types";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";

const PipelineCanvasNodeSink = memo(({ id, data, selected }: PipelineCanvasNodeSinkProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "target" });
  const { removeNode } = usePipelineCanvasActions();
  const { selectNode } = usePipelineCanvasSelection();
  const { data: connectionsData } = useSuspenseListConnectionsQuery();
  const connection = connectionsData.connections.find((item) => item.id === data.connectionId);

  return (
    <PipelineCanvasNode
      connector={connection?.connector ?? ""}
      label={connection?.name ?? data.connectionId}
      kind={ConnectorKind.SINK}
      isConnected={connections.length > 0}
      isSelected={selected}
      onSettings={() => selectNode(id)}
      onDelete={isReadOnly ? undefined : () => removeNode(id)}
    />
  );
});

PipelineCanvasNodeSink.displayName = "PipelineCanvasNodeSink";

export default PipelineCanvasNodeSink;
