import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import {
  ConnectorKind,
  ReplicationMode,
  StandardSyncMode,
} from "@/gen/ingestion/v1/common_pb";
import {
  DiscoverResourcesRequestSchema,
  GetResourceColumnsRequestSchema,
  type Resource,
  type ResourceColumn,
} from "@/gen/ingestion/v1/connectors_pb";
import { PipelineEdgeSchema, PipelineNodeSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { getCursorOptions } from "@/pages/pipelines/components/create/rows";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import {
  useDiscoverResourcesQuery,
  useGetResourceColumnsQuery,
} from "@/api/queries/connectors";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

const intersectModes = (sets: StandardSyncMode[][]): StandardSyncMode[] => {
  if (!sets.length) return [];
  return sets.slice(1).reduce(
    (common, modes) => common.filter((mode) => modes.includes(mode)),
    sets[0] ?? [],
  );
};

export const usePipelineCanvasPanelResourceOptions = (edge: CanvasEdge) => {
  const connectionByNodeId = usePipelineCanvasConnections();
  const sourceConnection = connectionByNodeId.get(edge.source);
  const sinkConnection = connectionByNodeId.get(edge.target);
  const sourceConnectionId = sourceConnection?.id ?? "";
  const isCdc = sourceConnection?.replication === ReplicationMode.CDC;
  const edgeResource = getCanvasEdgeResource(edge);

  const { data: discovered, isLoading: isLoadingResources } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId: sourceConnectionId }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: sourceConnectionId !== "" && edgeResource === "" },
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
      enabled: sourceConnectionId !== "" && coveredResources.length > 0,
    },
  });

  const validationInput = useMemo(
    () =>
      create(ValidatePipelineRequestSchema, {
        nodes:
          sourceConnection && sinkConnection
            ? [
                create(PipelineNodeSchema, {
                  id: edge.source,
                  kind: ConnectorKind.SOURCE,
                  connectionId: sourceConnection.id,
                }),
                create(PipelineNodeSchema, {
                  id: edge.target,
                  kind: ConnectorKind.SINK,
                  connectionId: sinkConnection.id,
                }),
              ]
            : [],
        edges: [
          create(PipelineEdgeSchema, {
            fromNode: edge.source,
            toNode: edge.target,
            resource: edgeResource,
            standardSyncMode: edge.data?.standardSyncMode ?? StandardSyncMode.UNSPECIFIED,
            cursors: edge.data?.cursors ?? [],
          }),
        ],
      }),
    [sourceConnection, sinkConnection, edge, edgeResource],
  );

  const { data: validation, isLoading: isLoadingValidation } = useValidatePipelineQuery({
    input: validationInput,
    options: {
      ...PROBE_QUERY_OPTIONS,
      enabled: !!sourceConnection && !!sinkConnection,
    },
  });

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

  const recommendedCursorByResource = useMemo<Record<Resource["name"], ResourceColumn["name"]>>(
    () =>
      Object.fromEntries(
        coveredResources.map((resource) => [
          resource,
          (columnsByResource.get(resource) ?? []).find((column) => column.isCursorRecommended)
            ?.name ?? "",
        ]),
      ),
    [coveredResources, columnsByResource],
  );

  const syncModeOptions = useMemo<StandardSyncMode[]>(() => {
    const verdict = validation?.edges[0];
    if (!verdict || isCdc) return [];
    if (!verdict.resources.length) return verdict.supportedModes;
    return intersectModes(verdict.resources.map((resource) => resource.supportedModes));
  }, [validation?.edges, isCdc]);

  const isLoading =
    isLoadingValidation ||
    (edgeResource === "" && isLoadingResources) ||
    (coveredResources.length > 0 && isPendingColumns && !isErrorColumns);

  return {
    isCdc,
    isLoading,
    coveredResources,
    syncModeOptions,
    cursorOptionsByResource,
    recommendedCursorByResource,
  };
};
