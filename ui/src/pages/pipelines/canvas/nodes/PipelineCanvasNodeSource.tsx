import { memo, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { DiscoverResourcesRequestSchema } from "@/gen/ingestion/v1/connectors_pb";

import { CONNECTOR_KIND_TO_HANDLE_ID_MAP } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
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

import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import { useDiscoverResourcesQuery } from "@/api/queries/connectors";

const useSourceResources = (connectionId: Connection["id"]) => {
  const { data, error, isFetching, refetch } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId }),
    options: { enabled: connectionId !== "", retry: false, networkMode: "always" },
  });

  const tables = useMemo<PipelineCanvasNodeTableInfo[]>(
    () =>
      data?.resources.map((resource) => ({
        name: resource.name,
        isConnected: false,
        hasTransform: false,
        isInvalid: false,
      })) ?? [],
    [data?.resources],
  );

  return { tables, error, isLoading: isFetching, refresh: () => void refetch() };
};

const PipelineCanvasNodeSource = memo(({ id, data, selected }: PipelineCanvasNodeSourceProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "source" });
  const { removeNode } = usePipelineCanvasActions();
  const { selectNode } = usePipelineCanvasSelection();
  const { data: connectionsData } = useSuspenseListConnectionsQuery();
  const connection = connectionsData.connections.find((item) => item.id === data.connectionId);
  const {
    tables: discoveredTables,
    error,
    isLoading,
    refresh,
  } = useSourceResources(data.connectionId);

  const connectedHandleIds = useMemo(
    () => new Set(connections.map((connection) => connection.sourceHandle)),
    [connections],
  );

  const { edges } = usePipelineCanvasState();
  const { invalidEdgeIds } = usePipelineCanvasValidation();
  const tables = useMemo(() => {
    const own = edges.filter((edge) => edge.source === id && edge.data?.transform !== undefined);
    const transformed = new Map(
      own.map((edge) => [edge.sourceHandle, invalidEdgeIds.has(edge.id)]),
    );
    return discoveredTables.map((table) => ({
      ...table,
      isConnected: connectedHandleIds.has(table.name),
      hasTransform: transformed.has(table.name),
      isInvalid: transformed.get(table.name) ?? false,
    }));
  }, [discoveredTables, connectedHandleIds, edges, id, invalidEdgeIds]);

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
});

PipelineCanvasNodeSource.displayName = "PipelineCanvasNodeSource";

export default PipelineCanvasNodeSource;
