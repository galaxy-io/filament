import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import { ConnectorKind, ReadMode, ReplicationMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import {
  DiscoverResourcesRequestSchema,
  GetResourceColumnsRequestSchema,
} from "@/gen/ingestion/v1/connectors_pb";
import { PipelineEdgeSchema, PipelineNodeSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE } from "@/pages/pipelines/components/create/constants";
import {
  buildResourceRowsBySink,
  buildSinkRows,
  getIssuesBySink,
  getSelectedCountBySink,
} from "@/pages/pipelines/components/create/rows";
import type { CreatePipelineModalState } from "@/pages/pipelines/components/create/types";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { useDiscoverResourcesQuery, useGetResourceColumnsQuery } from "@/api/queries/connectors";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

export const useCreatePipelineResources = (state: CreatePipelineModalState) => {
  const source = state.sourceConnection;
  const connectionId = source?.id ?? "";
  const replication = source?.replication ?? ReplicationMode.UNSPECIFIED;
  const isCdc = replication === ReplicationMode.CDC;

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

  const {
    data: columns,
    isPending: isPendingColumns,
    isError: isErrorColumns,
  } = useGetResourceColumnsQuery({
    input: create(GetResourceColumnsRequestSchema, { connectionId, resources: resourceNames }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: connectionId !== "" && resourceNames.length > 0 },
  });

  const validationInput = useMemo(
    () =>
      create(ValidatePipelineRequestSchema, {
        nodes: source
          ? [
              create(PipelineNodeSchema, {
                id: source.id,
                kind: ConnectorKind.SOURCE,
                connectionId: source.id,
              }),
              ...state.sinkConnections.map((sink) =>
                create(PipelineNodeSchema, {
                  id: sink.id,
                  kind: ConnectorKind.SINK,
                  connectionId: sink.id,
                }),
              ),
            ]
          : [],
        edges: source
          ? state.sinkConnections.map((sink) =>
              create(PipelineEdgeSchema, {
                fromNode: source.id,
                toNode: sink.id,
                readMode: isCdc ? ReadMode.UNSPECIFIED : ReadMode.FULL,
                writeMode: isCdc
                  ? WriteMode.UNSPECIFIED
                  : (state.sinkWriteModes[sink.id] ?? CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE),
              }),
            )
          : [],
      }),
    [source, state.sinkConnections, state.sinkWriteModes, isCdc],
  );

  const { data: validation, isLoading: isLoadingValidation } = useValidatePipelineQuery({
    input: validationInput,
    options: {
      ...PROBE_QUERY_OPTIONS,
      enabled: connectionId !== "" && state.sinkConnections.length > 0,
    },
  });

  const supportedReadModesBySink = useMemo(
    () =>
      Object.fromEntries(
        (validation?.edges ?? []).map((edge) => [
          edge.toNode,
          Object.fromEntries(
            edge.resources.map((resource) => [resource.resource, resource.supportedReadModes]),
          ),
        ]),
      ),
    [validation?.edges],
  );

  const supportedWriteModesBySink = useMemo(
    () =>
      Object.fromEntries(
        (validation?.edges ?? []).map((edge) => [edge.toNode, edge.supportedWriteModes]),
      ),
    [validation?.edges],
  );

  const isLoading =
    isLoadingResources ||
    isLoadingValidation ||
    (resourceNames.length > 0 && isPendingColumns && !isErrorColumns);

  const rowsBySink = useMemo(
    () =>
      isLoading
        ? {}
        : buildResourceRowsBySink({
            state,
            resources,
            columns,
            supportedReadModesBySink,
            isCdc,
          }),
    [isLoading, state, resources, columns, supportedReadModesBySink, isCdc],
  );

  const sinks = useMemo(
    () => buildSinkRows({ state, rowsBySink, isCdc, supportedWriteModesBySink }),
    [state, rowsBySink, isCdc, supportedWriteModesBySink],
  );

  const issuesBySink = useMemo(
    () => (isLoading || discoverError ? {} : getIssuesBySink(rowsBySink, sinks)),
    [rowsBySink, sinks, isLoading, discoverError],
  );
  const selectedCountBySink = useMemo(() => getSelectedCountBySink(rowsBySink), [rowsBySink]);

  return {
    rowsBySink,
    sinks,
    replication,
    isCdc,
    issuesBySink,
    selectedCountBySink,
    isLoading,
    discoverError,
  };
};
