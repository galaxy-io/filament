import { memo, useEffect, useMemo, useRef } from "react";

import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";
import { PIPELINE_NODE_SINK_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import { isConnectionNode } from "@/pages/pipelines/canvas/graph";
import PipelineNode from "@/pages/pipelines/canvas/nodes/PipelineNode";
import PipelineNodeConfigIsland from "@/pages/pipelines/canvas/nodes/PipelineNodeConfigIsland";
import type { PipelineNodeSinkProps } from "@/pages/pipelines/canvas/nodes/types";
import { usePipelineNodeActions } from "@/pages/pipelines/canvas/nodes/usePipelineNodeActions";
import {
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

import { normalizeIdentifier } from "@/utils/naming";

const PipelineNodeSink = memo(({ id, data, selected }: PipelineNodeSinkProps) => {
  const state = usePipelineCanvasState();
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "target" });
  const { isConfigOpen, toggleConfigOpen, removeNode, setNodeConfig } = usePipelineNodeActions(id);

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

  // Mirror the server's create-time schema defaulting into the config once,
  // as soon as the spec resolves, so the destination is visible and editable.
  const schemaField = useConnectorSpec(data.connector, ConnectorKind.SINK)?.schemaField;
  const populated = useRef(false);
  // biome-ignore lint/correctness/useExhaustiveDependencies: populate once
  useEffect(() => {
    if (populated.current || !schemaField || !defaultSchema || isReadOnly) return;
    populated.current = true;
    const config = data.config ?? {};
    const current = config[schemaField];
    if (typeof current === "string" && current !== "") return;
    setNodeConfig({ ...config, [schemaField]: defaultSchema });
  }, [schemaField, defaultSchema]);

  return (
    <PipelineNode
      connector={data.connector}
      label={data.label}
      kind={ConnectorKind.SINK}
      handleId={PIPELINE_NODE_SINK_HANDLE_ID}
      isConnected={connections.length > 0}
      isSelected={selected}
      onDelete={isReadOnly ? undefined : removeNode}
      onConfigure={toggleConfigOpen}
    >
      {isConfigOpen && (
        <PipelineNodeConfigIsland
          connector={data.connector}
          kind={ConnectorKind.SINK}
          config={data.config}
          onChange={setNodeConfig}
          isSelected={selected}
          isDisabled={isReadOnly}
        />
      )}
    </PipelineNode>
  );
});

PipelineNodeSink.displayName = "PipelineNodeSink";

export default PipelineNodeSink;
