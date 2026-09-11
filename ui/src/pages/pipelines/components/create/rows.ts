import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type {
  GetResourceColumnsResponse,
  Resource,
  ResourceColumn,
} from "@/gen/ingestion/v1/connectors_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import {
  CREATE_PIPELINE_MODAL_DEFAULT_READ_MODE,
  CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE,
} from "@/pages/pipelines/components/create/constants";
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

const writeModesForRead = (readMode: ReadMode): WriteMode[] => {
  if (readMode === ReadMode.INCREMENTAL) return [WriteMode.APPEND, WriteMode.UPSERT];
  return [WriteMode.APPEND, WriteMode.REPLACE, WriteMode.UPSERT];
};

const getCompatibleWriteModes = (readModes: ReadMode[]): WriteMode[] =>
  readModes.length
    ? readModes
        .map(writeModesForRead)
        .reduce((left, right) => left.filter((mode) => right.includes(mode)))
    : [WriteMode.APPEND, WriteMode.REPLACE, WriteMode.UPSERT];

const getResourceStatus = ({
  readMode,
  readModeOptions,
  cursorField,
  cursorOptions,
  isCursorKnown,
  hasPrimaryKey,
  needsPrimaryKey,
  resource,
}: {
  readMode: ReadMode;
  readModeOptions: ReadMode[];
  cursorField: ResourceColumn["name"];
  cursorOptions: ResourceColumn[];
  isCursorKnown: boolean;
  hasPrimaryKey: boolean;
  needsPrimaryKey: boolean;
  resource: Resource["name"];
}): CreatePipelineModalResourceStatus | undefined => {
  if (!readModeOptions.includes(readMode)) {
    return {
      message: `${resource} does not support the selected read mode`,
      isBlocking: true,
    };
  }
  if (readMode === ReadMode.INCREMENTAL && !cursorField) {
    if (cursorOptions.length) {
      return {
        message: `Incremental reads require a cursor column for ${resource}`,
        isBlocking: true,
      };
    }
    if (isCursorKnown) {
      return {
        message: `${resource} has no usable cursor column; read it in full instead`,
        isBlocking: true,
      };
    }
  }
  if (needsPrimaryKey && !hasPrimaryKey) {
    return {
      message: `Upsert requires a primary key, but none was discovered for ${resource}`,
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
  supportedReadModes,
  isCdc,
}: {
  state: CreatePipelineModalState;
  sinkId: Connection["id"];
  resources: Resource[];
  columns: GetResourceColumnsResponse | undefined;
  supportedReadModes: Record<Resource["name"], ReadMode[]>;
  isCdc: boolean;
}): CreatePipelineModalResourceRow[] => {
  const columnsByResource = new Map(
    (columns?.resources ?? []).map((entry) => [entry.resource, entry.columns]),
  );
  const needsPrimaryKey =
    (state.sinkWriteModes[sinkId] ?? CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE) === WriteMode.UPSERT;

  return resources.map((resource) => {
    const resourceColumns = columnsByResource.get(resource.name);
    const isCursorKnown = resourceColumns !== undefined;
    const cursorOptions = getCursorOptions(resourceColumns ?? []);
    const autoCursor =
      (resourceColumns ?? []).find((column) => column.isCursorRecommended)?.name ?? "";
    const readModeOptions = isCdc ? [] : (supportedReadModes[resource.name] ?? []);
    const defaultReadMode =
      autoCursor && readModeOptions.includes(ReadMode.INCREMENTAL)
        ? ReadMode.INCREMENTAL
        : CREATE_PIPELINE_MODAL_DEFAULT_READ_MODE;
    const readMode = state.resourceReadModes[sinkId]?.[resource.name] ?? defaultReadMode;
    const isSelected =
      state.resourceSelection[sinkId]?.[resource.name] ??
      resource.metadata.default_resources !== "false";
    const cursorField = state.resourceCursors[sinkId]?.[resource.name] ?? autoCursor;

    return {
      name: resource.name,
      displayName: resource.displayName || resource.name,
      isSelectable: resource.isSelectable,
      isSelected: resource.isSelectable && isSelected,
      readMode,
      readModeOptions,
      cursorField,
      cursorOptions,
      status:
        isCdc || !isSelected
          ? undefined
          : getResourceStatus({
              readMode,
              readModeOptions,
              cursorField,
              cursorOptions,
              isCursorKnown,
              hasPrimaryKey: resource.primaryKey.length > 0,
              needsPrimaryKey,
              resource: resource.name,
            }),
    };
  });
};

export const buildResourceRowsBySink = ({
  state,
  resources,
  columns,
  supportedReadModesBySink,
  isCdc,
}: {
  state: CreatePipelineModalState;
  resources: Resource[];
  columns: GetResourceColumnsResponse | undefined;
  supportedReadModesBySink: Record<Connection["id"], Record<Resource["name"], ReadMode[]>>;
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
        supportedReadModes: supportedReadModesBySink[sink.id] ?? {},
        isCdc,
      }),
    ]),
  );

export const buildSinkRows = ({
  state,
  rowsBySink,
  isCdc,
  supportedWriteModesBySink,
}: {
  state: CreatePipelineModalState;
  rowsBySink: Record<Connection["id"], CreatePipelineModalResourceRow[]>;
  isCdc: boolean;
  supportedWriteModesBySink: Record<Connection["id"], WriteMode[]>;
}): CreatePipelineModalSinkRow[] =>
  state.sinkConnections.map((connection) => {
    const readModes = [
      ...new Set(
        (rowsBySink[connection.id] ?? [])
          .filter((row) => row.isSelected)
          .map((row) => row.readMode),
      ),
    ];
    const supported = supportedWriteModesBySink[connection.id] ?? [];
    const writeModeOptions = isCdc
      ? supported
      : supported.filter((mode) => getCompatibleWriteModes(readModes).includes(mode));
    const stored =
      state.sinkWriteModes[connection.id] ??
      (isCdc ? WriteMode.APPEND : CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE);
    const writeMode = writeModeOptions.includes(stored)
      ? stored
      : (writeModeOptions[0] ?? WriteMode.UNSPECIFIED);
    return {
      connection,
      writeMode,
      writeModeOptions,
    };
  });

export const getIssuesBySink = (
  rowsBySink: Record<Connection["id"], CreatePipelineModalResourceRow[]>,
  sinks: CreatePipelineModalSinkRow[],
): Record<Connection["id"], string[]> =>
  Object.fromEntries(
    sinks.map((sink) => {
      const rows = rowsBySink[sink.connection.id] ?? [];
      if (!rows.some((row) => row.isSelected))
        return [sink.connection.id, ["No resources selected"]];
      const issues = rows
        .filter((row) => row.status?.isBlocking)
        .map((row) => row.status?.message ?? "")
        .filter(Boolean);
      if (!sink.writeModeOptions.length)
        issues.push("No write mode supports the selected read modes");
      return [sink.connection.id, [...new Set(issues)]];
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
