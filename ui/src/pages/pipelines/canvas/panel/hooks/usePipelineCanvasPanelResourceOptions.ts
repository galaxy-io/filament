import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { GetConnectionCapabilitiesRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import {
  ConnectorKind,
  ReadMode,
  ReplicationMode,
  type WriteMode,
} from "@/gen/ingestion/v1/common_pb";
import {
  DiscoverResourcesRequestSchema,
  GetResourceColumnsRequestSchema,
  type Resource,
  type ResourceColumn,
} from "@/gen/ingestion/v1/connectors_pb";

import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { CREATE_PIPELINE_MODAL_FALLBACK_READ_MODES } from "@/pages/pipelines/components/create/constants";
import {
  getCompatibleWriteModes,
  getCursorOptions,
  getSinkWriteModes,
} from "@/pages/pipelines/components/create/rows";

import { useGetConnectionCapabilitiesQuery } from "@/api/queries/capabilities";
import {
  useDiscoverResourcesQuery,
  useGetResourceColumnsQuery,
  useListConnectorsQuery,
} from "@/api/queries/connectors";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

export const usePipelineCanvasPanelResourceOptions = (edge: CanvasEdge) => {
  const connectionByNodeId = usePipelineCanvasConnections();

  const sourceConnectionId = connectionByNodeId.get(edge.source)?.id ?? "";
  const sinkConnection = connectionByNodeId.get(edge.target);

  const { data: capabilities, isLoading: isLoadingCapabilities } =
    useGetConnectionCapabilitiesQuery({
      input: create(GetConnectionCapabilitiesRequestSchema, { id: sourceConnectionId }),
      options: { ...PROBE_QUERY_OPTIONS, enabled: sourceConnectionId !== "" },
    });

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

  const { data: connectors } = useListConnectorsQuery();
  const sinkSpec = connectors?.connectors.find(
    (connector) =>
      connector.name === sinkConnection?.connector && connector.kind === ConnectorKind.SINK,
  );

  const isCdc = (capabilities?.replication ?? ReplicationMode.UNSPECIFIED) === ReplicationMode.CDC;

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

  const readModeOptions = useMemo<ReadMode[]>(() => {
    const connectionReadModes = capabilities?.readModes?.length
      ? capabilities.readModes
      : CREATE_PIPELINE_MODAL_FALLBACK_READ_MODES;
    const canIncremental = coveredResources.every(
      (resource) =>
        (cursorOptionsByResource[resource] ?? []).length > 0 ||
        columnsByResource.get(resource) === undefined,
    );
    return connectionReadModes.filter((mode) => mode !== ReadMode.INCREMENTAL || canIncremental);
  }, [capabilities?.readModes, coveredResources, cursorOptionsByResource, columnsByResource]);

  const writeModesByReadMode = useMemo<Partial<Record<ReadMode, WriteMode[]>>>(
    () =>
      Object.fromEntries(
        (capabilities?.readModeWriteCompatibilities ?? []).map((entry) => [
          entry.readMode,
          entry.writeModes,
        ]),
      ),
    [capabilities?.readModeWriteCompatibilities],
  );

  const writeModeOptions = useMemo<WriteMode[]>(() => {
    const readMode = edge.data?.readMode ?? ReadMode.UNSPECIFIED;
    const compatible = getCompatibleWriteModes(
      readMode === ReadMode.UNSPECIFIED ? [] : [readMode],
      writeModesByReadMode,
    );
    const supported = getSinkWriteModes(sinkSpec);
    const narrowed = supported.filter((mode) => compatible.includes(mode));
    return narrowed.length ? narrowed : supported;
  }, [edge.data?.readMode, writeModesByReadMode, sinkSpec]);

  const isLoading =
    isLoadingCapabilities ||
    (edgeResource === "" && isLoadingResources) ||
    (coveredResources.length > 0 && isPendingColumns && !isErrorColumns);

  return {
    isCdc,
    isLoading,
    coveredResources,
    readModeOptions,
    writeModeOptions,
    cursorOptionsByResource,
    recommendedCursorByResource,
  };
};
