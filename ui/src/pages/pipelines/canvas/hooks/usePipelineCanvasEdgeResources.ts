import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { ExecutionMode, ReplicationMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  DiscoverResourcesRequestSchema,
  GetResourceColumnsRequestSchema,
  type Resource,
  type ResourceColumn,
} from "@/gen/ingestion/v1/connectors_pb";

import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { getCursorOptions, getDefaultCursor } from "@/pages/pipelines/components/resource/utils";
import { usePipelineExecutionMode } from "@/pages/pipelines/hooks/usePipelineExecutionMode";

import { useDiscoverResourcesQuery, useGetResourceColumnsQuery } from "@/api/queries/connectors";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

interface PipelineCanvasEdgeResourcesOptions {
  enabled?: boolean;
}

export interface PipelineCanvasEdgeResources {
  isContinuous: boolean;
  hasReadLevers: boolean;
  isTransformable: boolean;
  isLoadingColumns: boolean;
  sourceConnectionId: Connection["id"];
  resource: Resource["name"];
  columns: ResourceColumn[] | undefined;
  cursorOptions: ResourceColumn[];
  defaultCursor: ResourceColumn["name"];
  isManagedIncremental: boolean;
  hasPrimaryKey: boolean;
}

export const usePipelineCanvasEdgeResources = (
  edge: CanvasEdge,
  { enabled = true }: PipelineCanvasEdgeResourcesOptions = {},
): PipelineCanvasEdgeResources => {
  const executionMode = usePipelineExecutionMode();
  const isContinuous = executionMode === ExecutionMode.CONTINUOUS;
  const connectionByNodeId = usePipelineCanvasConnections();
  const sourceConnection = connectionByNodeId.get(edge.source);
  const sourceConnectionId = sourceConnection?.id ?? "";
  const isCdc = sourceConnection?.replication === ReplicationMode.CDC;
  const hasReadLevers = !isCdc && !isContinuous;
  const isTransformable = !isContinuous;
  const resource = getCanvasEdgeResource(edge);
  const isQueryable = enabled && sourceConnectionId !== "" && resource !== "";

  const { data: discovered } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId: sourceConnectionId }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: isQueryable },
  });

  const { data: columnsData, isLoading } = useGetResourceColumnsQuery({
    input: create(GetResourceColumnsRequestSchema, {
      connectionId: sourceConnectionId,
      resources: [resource],
    }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: isQueryable },
  });

  const entry = columnsData?.resources.find((candidate) => candidate.resource === resource);
  const columns = entry?.columns;
  const isManagedIncremental = entry?.managedIncremental ?? false;
  const cursorOptions = useMemo(() => getCursorOptions(columns ?? []), [columns]);

  return {
    isContinuous,
    hasReadLevers,
    isTransformable,
    isLoadingColumns: isQueryable && isLoading,
    sourceConnectionId,
    resource,
    columns,
    cursorOptions,
    defaultCursor: isManagedIncremental ? "" : getDefaultCursor(columns ?? []),
    isManagedIncremental,
    hasPrimaryKey:
      (discovered?.resources.find((candidate) => candidate.name === resource)?.primaryKey.length ??
        0) > 0,
  };
};
