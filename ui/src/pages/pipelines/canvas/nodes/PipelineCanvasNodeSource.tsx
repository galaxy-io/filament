import { memo, useCallback, useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import { DiscoverResourcesRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { CONNECTOR_KIND_TO_HANDLE_ID_MAP } from "@/pages/pipelines/canvas/constants";
import PipelineCanvasNode from "@/pages/pipelines/canvas/nodes/PipelineCanvasNode";
import PipelineCanvasNodeConfigIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeConfigIsland";
import PipelineCanvasNodeSourceIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeSourceIsland";
import type { PipelineCanvasNodeSourceProps } from "@/pages/pipelines/canvas/nodes/types";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
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
      })) ?? [],
    [data?.resources],
  );

  return { tables, error, isLoading: isFetching, refresh: () => void refetch() };
};

interface PipelineCanvasNodeSourceState {
  isConfigOpen: boolean;
}

const DEFAULT_STATE: PipelineCanvasNodeSourceState = {
  isConfigOpen: false,
};

const PipelineCanvasNodeSource = memo(({ id, data, selected }: PipelineCanvasNodeSourceProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "source" });
  const { removeNode, setNodeConfig } = usePipelineCanvasActions();
  const [localState, setLocalState] = useState<PipelineCanvasNodeSourceState>(DEFAULT_STATE);
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

  const tables = useMemo(
    () =>
      discoveredTables.map((table) => ({
        ...table,
        isConnected: connectedHandleIds.has(table.name),
      })),
    [discoveredTables, connectedHandleIds],
  );

  const handleConfigure = useCallback(() => {
    setLocalState((prev) => ({ ...prev, isConfigOpen: !prev.isConfigOpen }));
  }, []);

  return (
    <PipelineCanvasNode
      connector={connection?.connector ?? ""}
      label={connection?.name ?? data.connectionId}
      kind={ConnectorKind.SOURCE}
      isConnected={connectedHandleIds.has(CONNECTOR_KIND_TO_HANDLE_ID_MAP[ConnectorKind.SOURCE])}
      isSelected={selected}
      onRefresh={isReadOnly ? undefined : refresh}
      onDelete={isReadOnly ? undefined : () => removeNode(id)}
      onConfigure={handleConfigure}
    >
      <PipelineCanvasNodeConfigIsland
        connector={connection?.connector ?? ""}
        kind={ConnectorKind.SOURCE}
        config={data.config}
        onChange={(config) => setNodeConfig(id, config)}
        isOpen={localState.isConfigOpen}
        isSelected={selected}
      />
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
