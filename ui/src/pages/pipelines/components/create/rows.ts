import { StandardSyncMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type {
  GetResourceColumnsResponse,
  Resource,
  ResourceColumn,
} from "@/gen/ingestion/v1/connectors_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import { CREATE_PIPELINE_MODAL_DEFAULT_SYNC_MODE } from "@/pages/pipelines/components/create/constants";
import type {
  CreatePipelineModalResourceRow,
  CreatePipelineModalResourceStatus,
  CreatePipelineModalSinkRow,
  CreatePipelineModalState,
} from "@/pages/pipelines/components/create/types";

export const getDefaultPipelineName = (
  source: Connection | null,
  sinks: Connection[],
): Pipeline["name"] => {
  const sinkNames = sinks.map((sink) => sink.name).join(", ");
  if (source && sinks.length) return `${source.name} -> ${sinkNames}`;
  if (source) return source.name;
  return sinkNames;
};

export const getCursorOptions = (columns: ResourceColumn[]): ResourceColumn[] =>
  columns
    .filter((column) => column.isCursorEligible)
    .sort((left, right) => {
      if (left.recommendationRank === right.recommendationRank) return 0;
      if (left.recommendationRank === 0) return 1;
      if (right.recommendationRank === 0) return -1;
      return left.recommendationRank - right.recommendationRank;
    });

const getResourceStatus = ({
  syncMode,
  syncModeOptions,
  cursorField,
  cursorOptions,
  isCursorKnown,
  resource,
}: {
  syncMode: StandardSyncMode;
  syncModeOptions: StandardSyncMode[];
  cursorField: ResourceColumn["name"];
  cursorOptions: ResourceColumn[];
  isCursorKnown: boolean;
  resource: Resource["name"];
}): CreatePipelineModalResourceStatus | undefined => {
  if (!syncModeOptions.includes(syncMode)) {
    return {
      message: syncModeOptions.length
        ? `${resource} does not support the default mode; choose an available mode`
        : `${resource} has no sync mode supported by this source and sink`,
      isBlocking: true,
    };
  }
  if (syncMode !== StandardSyncMode.INCREMENTAL || cursorField) return undefined;
  if (cursorOptions.length) {
    return {
      message: `Incremental sync requires a cursor column for ${resource}`,
      isBlocking: true,
    };
  }
  if (isCursorKnown) {
    return {
      message: `${resource} has no usable cursor column; use Replace or Append instead`,
      isBlocking: true,
    };
  }
  return undefined;
};

const buildResourceRows = ({
  state,
  sinkId,
  resources,
  columns,
  supportedModes,
  isCdc,
}: {
  state: CreatePipelineModalState;
  sinkId: Connection["id"];
  resources: Resource[];
  columns: GetResourceColumnsResponse | undefined;
  supportedModes: Record<Resource["name"], StandardSyncMode[]>;
  isCdc: boolean;
}): CreatePipelineModalResourceRow[] => {
  const columnsByResource = new Map(
    (columns?.resources ?? []).map((entry) => [entry.resource, entry.columns]),
  );

  return resources.map((resource) => {
    const resourceColumns = columnsByResource.get(resource.name);
    const isCursorKnown = resourceColumns !== undefined;
    const cursorOptions = getCursorOptions(resourceColumns ?? []);
    const autoCursor =
      (resourceColumns ?? []).find((column) => column.isCursorRecommended)?.name ?? "";
    const syncModeOptions = isCdc ? [] : (supportedModes[resource.name] ?? []);
    const storedMode = state.resourceSyncModes[sinkId]?.[resource.name];
    const defaultMode =
      autoCursor && syncModeOptions.includes(StandardSyncMode.INCREMENTAL)
        ? StandardSyncMode.INCREMENTAL
        : CREATE_PIPELINE_MODAL_DEFAULT_SYNC_MODE;
    const syncMode = storedMode ?? defaultMode;
    const isSelected = state.resourceSelection[sinkId]?.[resource.name] ?? true;
    const cursorField = state.resourceCursors[sinkId]?.[resource.name] ?? autoCursor;

    return {
      name: resource.name,
      displayName: resource.displayName || resource.name,
      isSelectable: resource.isSelectable,
      isSelected: resource.isSelectable && isSelected,
      syncMode,
      syncModeOptions,
      cursorField,
      cursorOptions,
      status:
        isCdc || !isSelected
          ? undefined
          : getResourceStatus({
              syncMode,
              syncModeOptions,
              cursorField,
              cursorOptions,
              isCursorKnown,
              resource: resource.name,
            }),
    };
  });
};

export const buildResourceRowsBySink = ({
  state,
  resources,
  columns,
  supportedModesBySink,
  isCdc,
}: {
  state: CreatePipelineModalState;
  resources: Resource[];
  columns: GetResourceColumnsResponse | undefined;
  supportedModesBySink: Record<
    Connection["id"],
    Record<Resource["name"], StandardSyncMode[]>
  >;
  isCdc: boolean;
}): Record<Connection["id"], CreatePipelineModalResourceRow[]> =>
  Object.fromEntries(
    state.sinkConnections.map((sink) => [
      sink.id,
      buildResourceRows({
        state,
        sinkId: sink.id,
        resources,
        columns,
        supportedModes: supportedModesBySink[sink.id] ?? {},
        isCdc,
      }),
    ]),
  );

export const buildSinkRows = (state: CreatePipelineModalState): CreatePipelineModalSinkRow[] =>
  state.sinkConnections.map((connection) => ({ connection }));

export const getIssuesBySink = (
  rowsBySink: Record<Connection["id"], CreatePipelineModalResourceRow[]>,
): Record<Connection["id"], string[]> =>
  Object.fromEntries(
    Object.entries(rowsBySink).map(([sinkId, rows]) => {
      if (!rows.some((row) => row.isSelected)) return [sinkId, ["No resources selected"]];

      return [
        sinkId,
        [
          ...new Set(
            rows
              .filter((row) => row.status?.isBlocking)
              .map((row) => row.status?.message ?? "")
              .filter(Boolean),
          ),
        ],
      ];
    }),
  );

export const getSelectedCountBySink = (
  rowsBySink: Record<Connection["id"], CreatePipelineModalResourceRow[]>,
): Record<Connection["id"], number> =>
  Object.fromEntries(
    Object.entries(rowsBySink).map(([sinkId, rows]) => [
      sinkId,
      rows.filter((row) => row.isSelected).length,
    ]),
  );
