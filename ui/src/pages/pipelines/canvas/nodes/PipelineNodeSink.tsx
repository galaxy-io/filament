import { memo, useMemo, useState } from "react";

import type { JsonValue } from "@bufbuild/protobuf";
import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { PIPELINE_NODE_SINK_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import { isConnectionNode } from "@/pages/pipelines/canvas/graph";
import PipelineNode from "@/pages/pipelines/canvas/nodes/PipelineNode";
import PipelineNodeConfigIsland from "@/pages/pipelines/canvas/nodes/PipelineNodeConfigIsland";
import type { PipelineNodeSinkProps } from "@/pages/pipelines/canvas/nodes/types";
import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/providers/canvas/actions";
import {
  usePipelineCanvasDispatch,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

import { normalizeIdentifier } from "@/utils/naming";

const PipelineNodeSink = memo(({ id, data, selected }: PipelineNodeSinkProps) => {
  const state = usePipelineCanvasState();
  const dispatch = usePipelineCanvasDispatch();
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "target" });
  const [isConfigOpen, setIsConfigOpen] = useState(false);

  // The schema this pipeline defaults to when the field is left empty: the
  // normalized name of the single upstream source connection, mirroring the
  // server's create-time defaulting.
  const defaultSchema = useMemo(() => {
    const sourceIds = new Set(
      state.edges.filter((edge) => edge.target === id).map((edge) => edge.source),
    );
    const labels = new Set(
      state.nodes
        .filter(isConnectionNode)
        .filter((node) => sourceIds.has(node.id))
        .map((node) => node.data.label),
    );
    if (labels.size !== 1) return undefined;
    return normalizeIdentifier([...labels][0] ?? "") || undefined;
  }, [state.edges, state.nodes, id]);

  const handleDelete = () => {
    dispatch({ type: PipelineCanvasActionType.REMOVE_NODE, payload: id });
  };

  const handleConfigChange = (config: Record<string, JsonValue>) => {
    dispatch({ type: PipelineCanvasActionType.SET_NODE_CONFIG, payload: { nodeId: id, config } });
  };

  return (
    <PipelineNode
      connector={data.connector}
      label={data.label}
      kind={ConnectorKind.SINK}
      handleId={PIPELINE_NODE_SINK_HANDLE_ID}
      isConnected={connections.length > 0}
      isSelected={selected}
      onDelete={isReadOnly ? undefined : handleDelete}
      onConfigure={() => setIsConfigOpen((open) => !open)}
    >
      {isConfigOpen && (
        <PipelineNodeConfigIsland
          connector={data.connector}
          kind={ConnectorKind.SINK}
          config={data.config}
          onChange={handleConfigChange}
          defaultSchema={defaultSchema}
          isSelected={selected}
          isDisabled={isReadOnly}
        />
      )}
    </PipelineNode>
  );
});

PipelineNodeSink.displayName = "PipelineNodeSink";

export default PipelineNodeSink;
