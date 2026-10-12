import { type FC, memo, useMemo } from "react";

import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { CONNECTOR_KIND_TO_HANDLE_ID_MAP } from "@/pages/pipelines/canvas/constants";
import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { usePipelineCanvasSourceResources } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSourceResources";
import { usePipelineCanvasValidation } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasValidation";
import PipelineCanvasNode from "@/pages/pipelines/canvas/nodes/PipelineCanvasNode";
import PipelineCanvasNodeSourceIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSourceIsland";
import type { PipelineCanvasNodeSourceProps } from "@/pages/pipelines/canvas/nodes/types";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { PipelineCanvasNodeTableInfo } from "@/pages/pipelines/canvas/types";

import { createGetConnectionInput, useGetConnectionQuery } from "@/api/queries/connections";

const PipelineCanvasNodeSource: FC<PipelineCanvasNodeSourceProps> = ({ id, data, selected }) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "source" });
  const { removeNode } = usePipelineCanvasActions();
  const { selectNode } = usePipelineCanvasSelection();
  const { data: connectionData } = useGetConnectionQuery({
    input: createGetConnectionInput(data.connectionId),
  });
  const connection = connectionData?.connection;
  const {
    names: discoveredNames,
    error,
    isLoading,
    refresh,
  } = usePipelineCanvasSourceResources(data.connectionId);

  const connectedHandleIds = useMemo(
    () => new Set(connections.map((connection) => connection.sourceHandle)),
    [connections],
  );

  const { edges } = usePipelineCanvasState();
  const { invalidEdgeIds } = usePipelineCanvasValidation();
  const tables = useMemo<PipelineCanvasNodeTableInfo[]>(() => {
    const nodeHandleId = CONNECTOR_KIND_TO_HANDLE_ID_MAP[ConnectorKind.SOURCE];
    const connectedNames = [...connectedHandleIds].filter(
      (name): name is string => !!name && name !== nodeHandleId,
    );
    const own = edges.filter((edge) => edge.source === id && edge.data?.transform !== undefined);
    const transformed = new Map<string, boolean>();
    for (const edge of own) {
      const resource = getCanvasEdgeResource(edge);
      transformed.set(
        resource,
        (transformed.get(resource) ?? false) || invalidEdgeIds.has(edge.id),
      );
    }
    return [...new Set([...discoveredNames, ...connectedNames])].map((name) => ({
      name,
      isConnected: connectedHandleIds.has(name),
      hasTransform: transformed.has(name),
      isInvalid: transformed.get(name) ?? false,
    }));
  }, [discoveredNames, connectedHandleIds, edges, id, invalidEdgeIds]);

  return (
    <PipelineCanvasNode
      connector={connection?.connector ?? ""}
      label={connection?.name ?? data.connectionId}
      kind={ConnectorKind.SOURCE}
      isConnected={connectedHandleIds.has(CONNECTOR_KIND_TO_HANDLE_ID_MAP[ConnectorKind.SOURCE])}
      isSelected={selected}
      onRefresh={isReadOnly ? undefined : refresh}
      onSettings={() => selectNode(id)}
      onDelete={isReadOnly ? undefined : () => removeNode(id)}
    >
      {(isLoading || error || tables.length > 0) && (
        <PipelineCanvasNodeSourceIsland
          tables={tables}
          error={error}
          isLoading={isLoading}
          isSelected={selected}
        />
      )}
    </PipelineCanvasNode>
  );
};

export default memo(PipelineCanvasNodeSource);
