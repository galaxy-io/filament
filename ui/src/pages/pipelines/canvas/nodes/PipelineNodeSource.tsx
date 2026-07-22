import { memo, useMemo } from "react";

import { useNodeConnections } from "@xyflow/react";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import { PIPELINE_NODE_SOURCE_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvas, useSourceResources } from "@/pages/pipelines/canvas/hooks";
import PipelineNode from "@/pages/pipelines/canvas/nodes/PipelineNode";
import PipelineNodeSourceIsland from "@/pages/pipelines/canvas/nodes/PipelineNodeSourceIsland";
import type { PipelineNodeSourceProps } from "@/pages/pipelines/canvas/nodes/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

const PipelineNodeSource = memo(({ id, data, selected }: PipelineNodeSourceProps) => {
  const { state, dispatch } = usePipelineCanvas();
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
      onRefresh={state.isReadOnly ? undefined : refresh}
      onDelete={state.isReadOnly ? undefined : handleDelete}
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
