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
  edgeResource: Resource["name"];
  coveredResources: Resource["name"][];
  columnsByResource: Map<Resource["name"], ResourceColumn[]>;
  cursorOptionsByResource: Record<Resource["name"], ResourceColumn[]>;
  defaultCursorByResource: Record<Resource["name"], ResourceColumn["name"]>;
  primaryKeyByResource: Record<Resource["name"], Resource["primaryKey"]>;
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
  const edgeResource = getCanvasEdgeResource(edge);

  const { data: discovered, isLoading: isLoadingResources } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId: sourceConnectionId }),
    options: {
      ...PROBE_QUERY_OPTIONS,
      enabled: enabled && sourceConnectionId !== "",
    },
  });

  const coveredResources = useMemo<Resource["name"][]>(
    () =>
      edgeResource !== ""
        ? [edgeResource]
        : (discovered?.resources ?? [])
            .filter((resource) => resource.isSelectable)
            .map((resource) => resource.name),
    [edgeResource, discovered?.resources],
  );

  const {
    data: columns,
    isPending: isPendingColumns,
    isError: isErrorColumns,
  } = useGetResourceColumnsQuery({
    input: create(GetResourceColumnsRequestSchema, {
      connectionId: sourceConnectionId,
      resources: coveredResources,
    }),
    options: {
      ...PROBE_QUERY_OPTIONS,
      enabled: enabled && sourceConnectionId !== "" && coveredResources.length > 0,
    },
  });

  const primaryKeyByResource = useMemo<Record<Resource["name"], Resource["primaryKey"]>>(
    () =>
      Object.fromEntries(
        (discovered?.resources ?? []).map((resource) => [resource.name, resource.primaryKey]),
      ),
    [discovered?.resources],
  );

  const columnsByResource = useMemo(
    () => new Map((columns?.resources ?? []).map((entry) => [entry.resource, entry.columns])),
    [columns?.resources],
  );

  const cursorOptionsByResource = useMemo<Record<Resource["name"], ResourceColumn[]>>(
    () =>
      Object.fromEntries(
        coveredResources.map((resource) => [
          resource,
          getCursorOptions(columnsByResource.get(resource) ?? []),
        ]),
      ),
    [coveredResources, columnsByResource],
  );

  const defaultCursorByResource = useMemo<Record<Resource["name"], ResourceColumn["name"]>>(
    () =>
      Object.fromEntries(
        coveredResources.map((resource) => [
          resource,
          getDefaultCursor(columnsByResource.get(resource) ?? []),
        ]),
      ),
    [coveredResources, columnsByResource],
  );

  const isLoadingColumns =
    enabled &&
    ((edgeResource === "" && isLoadingResources) ||
      (coveredResources.length > 0 && isPendingColumns && !isErrorColumns));

  return {
    isContinuous,
    hasReadLevers,
    isTransformable,
    isLoadingColumns,
    sourceConnectionId,
    edgeResource,
    coveredResources,
    columnsByResource,
    cursorOptionsByResource,
    defaultCursorByResource,
    primaryKeyByResource,
  };
};
