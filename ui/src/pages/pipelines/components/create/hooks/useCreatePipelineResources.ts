import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { keepPreviousData } from "@tanstack/react-query";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import {
  ConnectorKind,
  ExecutionMode,
  ReadMode,
  ReplicationMode,
  WriteMode,
} from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  DiscoverResourcesRequestSchema,
  GetResourceColumnsRequestSchema,
  ResourceSchema,
} from "@/gen/ingestion/v1/connectors_pb";
import {
  PipelineEdgeSchema,
  PipelineGraphSchema,
  PipelineNodeSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import { CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE } from "@/pages/pipelines/components/create/constants";
import {
  buildResourceRowsBySink,
  buildSinkRows,
  getIssuesBySink,
  getSelectedCountBySink,
  isResourceSelected,
} from "@/pages/pipelines/components/create/rows";
import type { CreatePipelineModalState } from "@/pages/pipelines/components/create/types";
import { getEdgeValidationErrors } from "@/pages/pipelines/utils";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { useDiscoverResourcesQuery, useGetResourceColumnsQuery } from "@/api/queries/connectors";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

export const useCreatePipelineResources = (state: CreatePipelineModalState) => {
  const source = state.sourceConnection;
  const connectionId = source?.id ?? "";
  const replication = source?.replication ?? ReplicationMode.UNSPECIFIED;
  const isCdc = replication === ReplicationMode.CDC;
  const isContinuous = state.executionMode === ExecutionMode.CONTINUOUS;
  const hasReadLevers = !isCdc && !isContinuous;

  const {
    data: discovered,
    error: discoverError,
    isLoading: isLoadingResources,
  } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: connectionId !== "" },
  });

  const resources = useMemo(
    () => [
      ...(discovered?.resources ?? []),
      ...state.manualResources.map((name) =>
        create(ResourceSchema, { name, displayName: name, isSelectable: true }),
      ),
    ],
    [discovered?.resources, state.manualResources],
  );
  const resourceNames = useMemo(() => resources.map((resource) => resource.name), [resources]);

  const {
    data: columns,
    isPending: isPendingColumns,
    isError: isErrorColumns,
  } = useGetResourceColumnsQuery({
    input: create(GetResourceColumnsRequestSchema, { connectionId, resources: resourceNames }),
    options: {
      ...PROBE_QUERY_OPTIONS,
      enabled: hasReadLevers && connectionId !== "" && resourceNames.length > 0,
    },
  });

  const { sinkConnections, sinkWriteModes, nodeConfigs, resourceSelection, executionMode } = state;
  const validationInput = useMemo(() => {
    const buildSinkEdges = (sink: Connection) => {
      const writeMode =
        sinkWriteModes[sink.id] ??
        (hasReadLevers ? CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE : WriteMode.APPEND);
      const selected = isContinuous
        ? resources.filter((resource) => isResourceSelected(resourceSelection[sink.id], resource))
        : [];
      const edgeResources = selected.length ? selected.map((resource) => resource.name) : [""];
      return edgeResources.map((resource) =>
        create(PipelineEdgeSchema, {
          fromNode: source?.id,
          toNode: sink.id,
          resource,
          readMode: hasReadLevers ? ReadMode.FULL : ReadMode.UNSPECIFIED,
          writeMode,
        }),
      );
    };
    return create(ValidatePipelineRequestSchema, {
      executionMode,
      graph: create(PipelineGraphSchema, {
        nodes: source
          ? [
              create(PipelineNodeSchema, {
                id: source.id,
                kind: ConnectorKind.SOURCE,
                connectionId: source.id,
                config: nodeConfigs[source.id],
              }),
              ...sinkConnections.map((sink) =>
                create(PipelineNodeSchema, {
                  id: sink.id,
                  kind: ConnectorKind.SINK,
                  connectionId: sink.id,
                  config: nodeConfigs[sink.id],
                }),
              ),
            ]
          : [],
        edges: source ? sinkConnections.flatMap(buildSinkEdges) : [],
      }),
    });
  }, [
    source,
    sinkConnections,
    sinkWriteModes,
    nodeConfigs,
    resourceSelection,
    executionMode,
    resources,
    hasReadLevers,
    isContinuous,
  ]);

  const {
    data: validation,
    isLoading: isLoadingValidation,
    isFetching: isValidating,
    error: validationError,
  } = useValidatePipelineQuery({
    input: validationInput,
    options: {
      ...PROBE_QUERY_OPTIONS,
      placeholderData: keepPreviousData,
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
    (hasReadLevers && resourceNames.length > 0 && isPendingColumns && !isErrorColumns);

  const rowsBySink = useMemo(
    () =>
      isLoading
        ? {}
        : buildResourceRowsBySink({
            state,
            resources,
            columns,
            supportedReadModesBySink,
            hasReadLevers,
          }),
    [isLoading, state, resources, columns, supportedReadModesBySink, hasReadLevers],
  );

  const sinks = useMemo(
    () => buildSinkRows({ state, rowsBySink, hasReadLevers, supportedWriteModesBySink }),
    [state, rowsBySink, hasReadLevers, supportedWriteModesBySink],
  );

  const issuesBySink = useMemo(() => {
    if (isLoading || discoverError) return {};
    const issues = getIssuesBySink(rowsBySink, sinks);
    if (!isContinuous) return issues;
    const graphErrors = [
      ...(validationError ? [validationError.message] : []),
      ...(validation?.errors ?? []).map((error) => error.message),
    ];
    return Object.fromEntries(
      sinks.map((sink) => {
        const sinkId = sink.connection.id;
        const hasSelection = (rowsBySink[sinkId] ?? []).some((row) => row.isSelected);
        const edgeErrors = hasSelection
          ? [
              ...graphErrors,
              ...(validation?.edges ?? [])
                .filter((edge) => edge.toNode === sinkId)
                .flatMap(getEdgeValidationErrors),
            ]
          : [];
        return [sinkId, [...new Set([...(issues[sinkId] ?? []), ...edgeErrors])]];
      }),
    );
  }, [rowsBySink, sinks, isContinuous, validation, validationError, isLoading, discoverError]);
  const selectedCountBySink = useMemo(() => getSelectedCountBySink(rowsBySink), [rowsBySink]);

  return {
    rowsBySink,
    sinks,
    replication,
    hasReadLevers,
    issuesBySink,
    selectedCountBySink,
    isLoading,
    isValidating,
    discoverError,
  };
};
