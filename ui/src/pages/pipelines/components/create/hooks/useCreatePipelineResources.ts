import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import {
  ConnectorKind,
  ExecutionMode,
  ReadMode,
  ReplicationMode,
  WriteMode,
} from "@/gen/ingestion/v1/common_pb";
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
} from "@/pages/pipelines/components/create/rows";
import type { CreatePipelineModalState } from "@/pages/pipelines/components/create/types";
import { commonExecutionModes, streamResourceLabel } from "@/pages/pipelines/streaming";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { useDiscoverResourcesQuery, useGetResourceColumnsQuery } from "@/api/queries/connectors";
import { PROBE_QUERY_OPTIONS } from "@/api/queries/constants";

export const useCreatePipelineResources = (state: CreatePipelineModalState) => {
  const source = state.sourceConnection;
  const connectionId = source?.id ?? "";
  const replication = source?.replication ?? ReplicationMode.UNSPECIFIED;
  const isCdc = replication === ReplicationMode.CDC;
  const isContinuous = state.executionMode === ExecutionMode.CONTINUOUS;
  const explicitNames = useMemo(
    () => (isContinuous ? state.manualStreamResources : []),
    [state.manualStreamResources, isContinuous],
  );

  const {
    data: discovered,
    error: discoverError,
    isLoading: isLoadingResources,
  } = useDiscoverResourcesQuery({
    input: create(DiscoverResourcesRequestSchema, { connectionId }),
    options: { ...PROBE_QUERY_OPTIONS, enabled: connectionId !== "" },
  });

  const resources = useMemo(
    () =>
      isContinuous
        ? [
            ...(discovered?.resources ?? [])
              .filter((r) => r.isSelectable && !explicitNames.includes(r.name))
              .map((r) =>
                create(ResourceSchema, {
                  ...r,
                  metadata: { ...r.metadata, default_resources: "false" },
                }),
              ),
            ...explicitNames.map((name) =>
              create(ResourceSchema, {
                name,
                displayName: name,
                isSelectable: true,
                metadata: { default_resources: "false" },
              }),
            ),
          ]
        : (discovered?.resources ?? []),
    [discovered?.resources, explicitNames, isContinuous],
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
      enabled: !isContinuous && connectionId !== "" && resourceNames.length > 0,
    },
  });

  const validationInput = useMemo(
    () =>
      create(ValidatePipelineRequestSchema, {
        executionMode: state.executionMode,
        graph: create(PipelineGraphSchema, {
          nodes: source
            ? [
                create(PipelineNodeSchema, {
                  id: source.id,
                  kind: ConnectorKind.SOURCE,
                  connectionId: source.id,
                  config: state.nodeConfigs[source.id],
                }),
                ...state.sinkConnections.map((sink) =>
                  create(PipelineNodeSchema, {
                    id: sink.id,
                    kind: ConnectorKind.SINK,
                    connectionId: sink.id,
                    config: state.nodeConfigs[sink.id],
                  }),
                ),
              ]
            : [],
          edges: source
            ? state.sinkConnections.flatMap((sink) =>
                (isContinuous
                  ? (() => {
                      const selected = resourceNames.filter(
                        (name) => state.resourceSelection[sink.id]?.[name] === true,
                      );
                      return selected.length ? selected : [""];
                    })()
                  : [""]
                ).map((resource) =>
                  create(PipelineEdgeSchema, {
                    resource: isContinuous
                      ? (state.streamResourceEdits?.[resource]?.subject ?? resource).trim()
                      : resource,
                    destinationResource: isContinuous
                      ? (
                          state.streamResourceEdits?.[resource]?.label ??
                          streamResourceLabel(resource)
                        ).trim()
                      : undefined,
                    fromNode: source.id,
                    toNode: sink.id,
                    readMode: isCdc || isContinuous ? ReadMode.UNSPECIFIED : ReadMode.FULL,
                    writeMode:
                      isCdc || isContinuous
                        ? (state.sinkWriteModes[sink.id] ?? WriteMode.APPEND)
                        : (state.sinkWriteModes[sink.id] ??
                          CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE),
                  }),
                ),
              )
            : [],
        }),
      }),
    [
      source,
      state.sinkConnections,
      state.sinkWriteModes,
      state.executionMode,
      state.nodeConfigs,
      state.resourceSelection,
      state.streamResourceEdits,
      resourceNames,
      isCdc,
      isContinuous,
    ],
  );

  const {
    data: validation,
    isFetching: isLoadingValidation,
    error: validationError,
  } = useValidatePipelineQuery({
    input: validationInput,
    options: {
      ...PROBE_QUERY_OPTIONS,
      enabled: connectionId !== "" && state.sinkConnections.length > 0,
    },
  });

  const supportedExecutionModes = useMemo(
    () => commonExecutionModes(validation?.edges ?? []),
    [validation?.edges],
  );
  const executionModeError =
    supportedExecutionModes && !supportedExecutionModes.includes(state.executionMode)
      ? supportedExecutionModes.length
        ? "Select an execution mode supported by all selected connections"
        : "These connections have no compatible execution mode available on this server"
      : undefined;

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
    (!isContinuous && resourceNames.length > 0 && isPendingColumns && !isErrorColumns);

  const rowsBySink = useMemo(
    () =>
      buildResourceRowsBySink({
        state,
        resources,
        columns,
        supportedReadModesBySink,
        isCdc,
      }),
    [state, resources, columns, supportedReadModesBySink, isCdc],
  );

  const sinks = useMemo(
    () => buildSinkRows({ state, rowsBySink, isCdc, supportedWriteModesBySink }),
    [state, rowsBySink, isCdc, supportedWriteModesBySink],
  );

  const issuesBySink = useMemo(() => {
    if (!isContinuous && (isLoading || discoverError)) return {};
    if (executionModeError) {
      return Object.fromEntries(sinks.map((sink) => [sink.connection.id, [executionModeError]]));
    }
    const issues = getIssuesBySink(rowsBySink, sinks);
    if (isContinuous) {
      for (const sink of sinks) {
        const errors = [
          ...(validationError ? [validationError.message] : []),
          ...(validation?.errors ?? []).map((error) => error.message),
          ...(validation?.edges ?? [])
            .filter((edge) => edge.toNode === sink.connection.id)
            .flatMap((edge) => [
              ...edge.errors.map((error) => error.message),
              ...edge.requirements
                .filter((requirement) => requirement.blocking)
                .map((requirement) => requirement.message),
            ]),
        ];
        const selected = (rowsBySink[sink.connection.id] ?? []).filter((row) => row.isSelected);
        if (selected.some((row) => !row.subject?.trim() || !row.destinationResource?.trim()))
          errors.push("Selected resources need a label and subject/topic");
        if (new Set(selected.map((row) => row.subject?.trim())).size !== selected.length)
          errors.push("Each selected subject/topic must be unique");
        if (
          new Set(selected.map((row) => row.destinationResource?.trim())).size !== selected.length
        )
          errors.push("Each destination label must be unique");
        issues[sink.connection.id] = [
          ...new Set([...(issues[sink.connection.id] ?? []), ...errors]),
        ];
      }
    }
    return issues;
  }, [
    rowsBySink,
    sinks,
    isContinuous,
    validation,
    validationError,
    isLoading,
    discoverError,
    executionModeError,
  ]);
  const selectedCountBySink = useMemo(() => getSelectedCountBySink(rowsBySink), [rowsBySink]);

  return {
    supportedExecutionModes,
    executionModeError,
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
