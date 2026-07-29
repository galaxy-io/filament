import { memo, useState } from "react";

import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import PipelineCanvasNode from "@/pages/pipelines/canvas/nodes/PipelineCanvasNode";
import PipelineCanvasNodeConfigIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeConfigIsland";
import type { PipelineCanvasNodeSinkProps } from "@/pages/pipelines/canvas/nodes/types";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

const PipelineCanvasNodeSink = memo(({ id, data, selected }: PipelineCanvasNodeSinkProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "target" });
  const { removeNode, setNodeConfig } = usePipelineCanvasActions();
  const [isConfigOpen, setIsConfigOpen] = useState(false);

  return (
    <PipelineCanvasNode
      connector={data.connector}
      label={data.label}
      kind={ConnectorKind.SINK}
      isConnected={connections.length > 0}
      isSelected={selected}
      onDelete={isReadOnly ? undefined : () => removeNode(id)}
      onConfigure={() => setIsConfigOpen((open) => !open)}
    >
      <PipelineCanvasNodeConfigIsland
        connector={data.connector}
        kind={ConnectorKind.SINK}
        config={data.config}
        onChange={(config) => setNodeConfig(id, config)}
        isOpen={isConfigOpen}
        isSelected={selected}
      />
    </PipelineCanvasNode>
  );
});

PipelineCanvasNodeSink.displayName = "PipelineCanvasNodeSink";

export default PipelineCanvasNodeSink;
