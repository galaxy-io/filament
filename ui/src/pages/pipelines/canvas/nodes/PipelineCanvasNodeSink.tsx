import { memo, useCallback, useState } from "react";

import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import PipelineCanvasNode from "@/pages/pipelines/canvas/nodes/PipelineCanvasNode";
import PipelineCanvasNodeConfigIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeConfigIsland";
import type { PipelineCanvasNodeSinkProps } from "@/pages/pipelines/canvas/nodes/types";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

interface PipelineCanvasNodeSinkState {
  isConfigOpen: boolean;
}

const DEFAULT_STATE: PipelineCanvasNodeSinkState = {
  isConfigOpen: false,
};

const PipelineCanvasNodeSink = memo(({ id, data, selected }: PipelineCanvasNodeSinkProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "target" });
  const { removeNode, setNodeConfig } = usePipelineCanvasActions();
  const [state, setState] = useState<PipelineCanvasNodeSinkState>(DEFAULT_STATE);

  const handleConfigToggle = useCallback(() => {
    setState((prev) => ({ ...prev, isConfigOpen: !prev.isConfigOpen }));
  }, []);

  return (
    <PipelineCanvasNode
      connector={data.connector}
      label={data.label}
      kind={ConnectorKind.SINK}
      isConnected={connections.length > 0}
      isSelected={selected}
      onDelete={isReadOnly ? undefined : () => removeNode(id)}
    >
      <PipelineCanvasNodeConfigIsland
        connector={data.connector}
        kind={ConnectorKind.SINK}
        config={data.config}
        onChange={(config) => setNodeConfig(id, config)}
        isSelected={selected}
        isOpen={state.isConfigOpen}
        onToggle={handleConfigToggle}
      />
    </PipelineCanvasNode>
  );
});

PipelineCanvasNodeSink.displayName = "PipelineCanvasNodeSink";

export default PipelineCanvasNodeSink;
