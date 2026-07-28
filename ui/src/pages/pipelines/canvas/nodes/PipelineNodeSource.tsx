import { memo, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { useNodeConnections } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { DiscoverResourcesRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { PIPELINE_NODE_SOURCE_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import PipelineNode from "@/pages/pipelines/canvas/nodes/PipelineNode";
import PipelineNodeSourceIsland from "@/pages/pipelines/canvas/nodes/PipelineNodeSourceIsland";
import type { PipelineNodeSourceProps } from "@/pages/pipelines/canvas/nodes/types";
import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/providers/canvas/actions";
import {
  usePipelineCanvasDispatch,
  usePipelineCanvasReadOnly,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { PipelineSourceNodeTableInfo } from "@/pages/pipelines/canvas/types";

import { useDiscoverResourcesQuery } from "@/api/queries/connectors";

const useSourceResources = (connectionId: string) => {
  const { data, error, isFetching, refetch } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId }),
    options: { enabled: connectionId !== "", retry: false, networkMode: "always" },
  });

  const tables = useMemo<PipelineSourceNodeTableInfo[]>(
    () =>
      data?.resources.map((resource) => ({
        name: resource.name,
        isConnected: false,
      })) ?? [],
    [data?.resources],
  );

  return { tables, error, isLoading: isFetching, refresh: () => void refetch() };
};

const PipelineNodeSource = memo(({ id, data, selected }: PipelineNodeSourceProps) => {
  const dispatch = usePipelineCanvasDispatch();
  const isReadOnly = usePipelineCanvasReadOnly();
  const connections = useNodeConnections({ handleType: "source" });
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

  const handleDelete = () => {
    dispatch({ type: PipelineCanvasActionType.REMOVE_NODE, payload: id });
  };

  return (
    <PipelineNode
      connector={data.connector}
      label={data.label}
      kind={ConnectorKind.SOURCE}
      handleId={PIPELINE_NODE_SOURCE_HANDLE_ID}
      isConnected={connectedHandleIds.has(PIPELINE_NODE_SOURCE_HANDLE_ID)}
      isSelected={selected}
      onRefresh={isReadOnly ? undefined : refresh}
      onDelete={isReadOnly ? undefined : handleDelete}
    >
      {(isLoading || error || tables.length > 0) && (
        <PipelineNodeSourceIsland
          tables={tables}
          error={error}
          isLoading={isLoading}
          isSelected={selected}
        />
      )}
    </PipelineNode>
  );
});

PipelineNodeSource.displayName = "PipelineNodeSource";

export default PipelineNodeSource;
