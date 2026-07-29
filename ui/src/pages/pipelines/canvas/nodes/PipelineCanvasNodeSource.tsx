import { memo, useMemo, useState } from "react";

import { create } from "@bufbuild/protobuf";
import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
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

import { useDiscoverResourcesQuery } from "@/api/queries/connectors";

const useSourceResources = (connectionId: string) => {
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

const PipelineCanvasNodeSource = memo(({ id, data, selected }: PipelineCanvasNodeSourceProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "source" });
  const { removeNode, setNodeConfig } = usePipelineCanvasActions();
  const [isConfigOpen, setIsConfigOpen] = useState(false);
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

  return (
    <PipelineCanvasNode
      connector={data.connector}
      label={data.label}
      kind={ConnectorKind.SOURCE}
      isConnected={connectedHandleIds.has(CONNECTOR_KIND_TO_HANDLE_ID_MAP[ConnectorKind.SOURCE])}
      isSelected={selected}
      onRefresh={isReadOnly ? undefined : refresh}
      onDelete={isReadOnly ? undefined : () => removeNode(id)}
      onConfigure={() => setIsConfigOpen((open) => !open)}
    >
      <PipelineCanvasNodeConfigIsland
        connector={data.connector}
        kind={ConnectorKind.SOURCE}
        config={data.config}
        onChange={(config) => setNodeConfig(id, config)}
        isOpen={isConfigOpen}
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
