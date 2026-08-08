import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { GetConnectionCapabilitiesRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import { ConnectorKind, ReplicationMode, type WriteMode } from "@/gen/ingestion/v1/common_pb";
import {
  DiscoverResourcesRequestSchema,
  GetResourceColumnsRequestSchema,
} from "@/gen/ingestion/v1/providers_pb";

import type { CreatePipelineModalState } from "@/pages/pipelines/components/create/types";
import {
  buildResourceRowsBySink,
  buildSinkRows,
  getBlockingMessages,
  getSelectedCountBySink,
  getSinkWriteModes,
} from "@/pages/pipelines/components/create/utils";

import { useGetConnectionCapabilitiesQuery } from "@/api/queries/capabilities";
import {
  useDiscoverResourcesQuery,
  useGetResourceColumnsQuery,
  useListConnectorsQuery,
} from "@/api/queries/connectors";

const PROBE_QUERY_OPTIONS = {
  retry: false,
  networkMode: "always",
  staleTime: Number.POSITIVE_INFINITY,
  refetchOnWindowFocus: false,
} as const;

export const useCreatePipelineResources = (state: CreatePipelineModalState) => {
  const connectionId = state.sourceConnection?.id ?? "";

  const { data: capabilities, isLoading: isLoadingCapabilities } =
    useGetConnectionCapabilitiesQuery({
      input: create(GetConnectionCapabilitiesRequestSchema, { id: connectionId }),
      options: { ...PROBE_QUERY_OPTIONS, enabled: connectionId !== "" },
    });

  const {
    data: discovered,
    error: discoverError,
    isLoading: isLoadingResources,
  } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: connectionId !== "" },
  });

  const resources = useMemo(() => discovered?.resources ?? [], [discovered?.resources]);
  const resourceNames = useMemo(() => resources.map((resource) => resource.name), [resources]);

  const { data: columns, isLoading: isLoadingColumns } = useGetResourceColumnsQuery({
    input: create(GetResourceColumnsRequestSchema, { connectionId, resources: resourceNames }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: connectionId !== "" && resourceNames.length > 0 },
  });

  const { data: connectors } = useListConnectorsQuery();

  const replication = capabilities?.replication ?? ReplicationMode.UNSPECIFIED;
  const isCdc = replication === ReplicationMode.CDC;

  const writeModesBySink = useMemo<Record<string, WriteMode[]>>(
    () =>
      Object.fromEntries(
        state.sinkConnections.map((sink) => [
          sink.id,
          getSinkWriteModes(
            connectors?.connectors.find(
              (connector) =>
                connector.name === sink.connector && connector.kind === ConnectorKind.SINK,
            ),
          ),
        ]),
      ),
    [state.sinkConnections, connectors?.connectors],
  );

  const rowsBySink = useMemo(
    () =>
      buildResourceRowsBySink({
        state,
        resources,
        columns,
        readModes: capabilities?.readModes ?? [],
        isCdc,
        writeModesBySink,
      }),
    [state, resources, columns, capabilities?.readModes, isCdc, writeModesBySink],
  );

  const sinks = useMemo(
    () => buildSinkRows({ state, rowsBySink, isCdc, writeModesBySink }),
    [state, rowsBySink, isCdc, writeModesBySink],
  );

  const blockingMessages = useMemo(() => getBlockingMessages(rowsBySink), [rowsBySink]);
  const blockingSinkIds = useMemo(
    () =>
      Object.entries(rowsBySink)
        .filter(([, rows]) => rows.some((row) => row.status?.isBlocking))
        .map(([sinkId]) => sinkId),
    [rowsBySink],
  );
  const selectedCountBySink = useMemo(() => getSelectedCountBySink(rowsBySink), [rowsBySink]);
  const hasEmptySink = state.sinkConnections.some(
    (sink) => (selectedCountBySink[sink.id] ?? 0) === 0,
  );

  return {
    rowsBySink,
    sinks,
    replication,
    isCdc,
    blockingMessages,
    blockingSinkIds,
    selectedCountBySink,
    hasEmptySink,
    isLoading: isLoadingCapabilities || isLoadingResources || isLoadingColumns,
    discoverError,
  };
};
