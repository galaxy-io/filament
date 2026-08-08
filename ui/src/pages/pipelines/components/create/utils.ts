import { create } from "@bufbuild/protobuf";

import { ConnectorKind, ReadMode, ReplicationMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type CreatePipelineRequest,
  CreatePipelineRequestSchema,
  type CreatePipelineVersionRequest,
  CreatePipelineVersionRequestSchema,
  type PipelineEdge,
  type PipelineNode,
} from "@/gen/ingestion/v1/pipelines_pb";
import type {
  ConnectorSpec,
  GetResourceColumnsResponse,
  Resource,
  ResourceColumn,
} from "@/gen/ingestion/v1/providers_pb";

import {
  CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE,
  CREATE_PIPELINE_MODAL_FALLBACK_READ_MODES,
  CREATE_PIPELINE_MODAL_FALLBACK_WRITE_MODES,
  READ_MODE_TO_WRITE_MODES_MAP,
} from "@/pages/pipelines/components/create/constants";
import type {
  CreatePipelineModalResourceRow,
  CreatePipelineModalResourceStatus,
  CreatePipelineModalSinkRow,
  CreatePipelineModalState,
} from "@/pages/pipelines/components/create/types";
import { mapPipelineScheduleStateToCron } from "@/pages/pipelines/settings/utils";

export const getDefaultPipelineName = (source: Connection | null, sinks: Connection[]): string => {
  const sinkNames = sinks.map((sink) => sink.name).join(", ");
  if (source && sinks.length) return `${source.name} -> ${sinkNames}`;
  if (source) return source.name;
  return sinkNames;
};

export const getSinkWriteModes = (spec: ConnectorSpec | undefined): WriteMode[] => {
  if (!spec?.capabilities) return CREATE_PIPELINE_MODAL_FALLBACK_WRITE_MODES;
  const isUpsertable =
    spec.capabilities.upsertable ||
    spec.capabilities.writePolicies.some((policy) => policy.mode === WriteMode.UPSERT);
  return isUpsertable
    ? [...CREATE_PIPELINE_MODAL_FALLBACK_WRITE_MODES, WriteMode.UPSERT]
    : CREATE_PIPELINE_MODAL_FALLBACK_WRITE_MODES;
};

const getCompatibleWriteModes = (readModes: ReadMode[]): WriteMode[] =>
  readModes.length
    ? readModes
        .map((readMode) => READ_MODE_TO_WRITE_MODES_MAP[readMode])
        .reduce((left, right) => left.filter((mode) => right.includes(mode)))
    : READ_MODE_TO_WRITE_MODES_MAP[ReadMode.UNSPECIFIED];

const getCursorOptions = (columns: ResourceColumn[]): ResourceColumn[] =>
  columns
    .filter((column) => column.cursorEligible)
    .sort((left, right) => {
      if (left.recommendationRank === right.recommendationRank) return 0;
      if (left.recommendationRank === 0) return 1;
      if (right.recommendationRank === 0) return -1;
      return left.recommendationRank - right.recommendationRank;
    });

const getResourceStatus = ({
  readMode,
  cursorField,
  cursorOptions,
  isCursorKnown,
  hasPrimaryKey,
  needsPrimaryKey,
  resource,
}: {
  readMode: ReadMode;
  cursorField: string;
  cursorOptions: ResourceColumn[];
  isCursorKnown: boolean;
  hasPrimaryKey: boolean;
  needsPrimaryKey: boolean;
  resource: string;
}): CreatePipelineModalResourceStatus | undefined => {
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
      message: `Writing by key needs a primary key, but none was discovered for ${resource}`,
      isBlocking: false,
    };
  }

  return undefined;
};

const buildResourceRows = ({
  state,
  sinkId,
  resources,
  columns,
  readModes,
  isCdc,
  writeModes,
}: {
  state: CreatePipelineModalState;
  sinkId: string;
  resources: Resource[];
  columns: GetResourceColumnsResponse | undefined;
  readModes: ReadMode[];
  isCdc: boolean;
  writeModes: WriteMode[];
}): CreatePipelineModalResourceRow[] => {
  const columnsByResource = new Map(
    (columns?.resources ?? []).map((entry) => [entry.resource, entry.columns]),
  );
  const connectionReadModes = readModes.length
    ? readModes
    : CREATE_PIPELINE_MODAL_FALLBACK_READ_MODES;
  const needsPrimaryKey = writeModes.includes(WriteMode.UPSERT);

  return resources.map((resource) => {
    const resourceColumns = columnsByResource.get(resource.name);
    const isCursorKnown = resourceColumns !== undefined;
    const cursorOptions = getCursorOptions(resourceColumns ?? []);
    const autoCursor =
      (resourceColumns ?? []).find((column) => column.cursorRecommended)?.name ?? "";
    const canIncremental =
      connectionReadModes.includes(ReadMode.INCREMENTAL) &&
      (cursorOptions.length > 0 || !isCursorKnown);

    const readModeOptions = isCdc
      ? []
      : connectionReadModes.filter((mode) => mode !== ReadMode.INCREMENTAL || canIncremental);

    const storedReadMode = state.resourceReadModes[sinkId]?.[resource.name];
    const defaultReadMode = autoCursor ? ReadMode.INCREMENTAL : ReadMode.FULL;
    const preferredReadMode = storedReadMode ?? defaultReadMode;
    const readMode = readModeOptions.includes(preferredReadMode)
      ? preferredReadMode
      : (readModeOptions[0] ?? ReadMode.FULL);

    const isSelected = state.resourceSelection[sinkId]?.[resource.name] ?? true;
    const cursorField = state.resourceCursors[sinkId]?.[resource.name] ?? autoCursor;

    return {
      name: resource.name,
      displayName: resource.displayName || resource.name,
      isSelectable: resource.selectable,
      isSelected: resource.selectable && isSelected,
      readMode,
      readModeOptions,
      cursorField,
      cursorOptions,
      status:
        isCdc || !isSelected
          ? undefined
          : getResourceStatus({
              readMode,
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
  readModes,
  isCdc,
  writeModesBySink,
}: {
  state: CreatePipelineModalState;
  resources: Resource[];
  columns: GetResourceColumnsResponse | undefined;
  readModes: ReadMode[];
  isCdc: boolean;
  writeModesBySink: Record<string, WriteMode[]>;
}): Record<string, CreatePipelineModalResourceRow[]> =>
  Object.fromEntries(
    state.sinkConnections.map((sink) => [
      sink.id,
      buildResourceRows({
        state,
        sinkId: sink.id,
        resources,
        columns,
        readModes,
        isCdc,
        writeModes: writeModesBySink[sink.id] ?? [],
      }),
    ]),
  );

export const buildSinkRows = ({
  state,
  rowsBySink,
  isCdc,
  writeModesBySink,
}: {
  state: CreatePipelineModalState;
  rowsBySink: Record<string, CreatePipelineModalResourceRow[]>;
  isCdc: boolean;
  writeModesBySink: Record<string, WriteMode[]>;
}): CreatePipelineModalSinkRow[] =>
  state.sinkConnections.map((connection) => {
    const rows = rowsBySink[connection.id] ?? [];
    const readModes = [...new Set(rows.filter((row) => row.isSelected).map((row) => row.readMode))];
    const compatible = getCompatibleWriteModes(readModes);
    const supported = writeModesBySink[connection.id] ?? CREATE_PIPELINE_MODAL_FALLBACK_WRITE_MODES;
    const narrowed = supported.filter((mode) => compatible.includes(mode));
    const writeModeOptions = narrowed.length ? narrowed : supported;

    const stored = state.sinkWriteModes[connection.id] ?? CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE;
    const writeMode = writeModeOptions.includes(stored)
      ? stored
      : (writeModeOptions[0] ?? CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE);

    return {
      connection,
      writeMode: isCdc ? WriteMode.UNSPECIFIED : writeMode,
      writeModeOptions,
    };
  });

export const getBlockingMessages = (
  rowsBySink: Record<string, CreatePipelineModalResourceRow[]>,
): string[] => [
  ...new Set(
    Object.values(rowsBySink)
      .flat()
      .filter((row) => row.status?.isBlocking)
      .map((row) => row.status?.message ?? "")
      .filter(Boolean),
  ),
];

export const getSelectedCountBySink = (
  rowsBySink: Record<string, CreatePipelineModalResourceRow[]>,
): Record<string, number> =>
  Object.fromEntries(
    Object.entries(rowsBySink).map(([sinkId, rows]) => [
      sinkId,
      rows.filter((row) => row.isSelected).length,
    ]),
  );

const buildNodes = (
  sourceConnection: Connection | null,
  sinks: CreatePipelineModalSinkRow[],
): PipelineNode[] => {
  if (!sourceConnection) return [];
  return [
    {
      id: sourceConnection.id,
      kind: ConnectorKind.SOURCE,
      connectionId: sourceConnection.id,
      secretRefs: {},
    },
    ...sinks.map((sink) => ({
      id: sink.connection.id,
      kind: ConnectorKind.SINK,
      connectionId: sink.connection.id,
      secretRefs: {},
    })),
  ] as PipelineNode[];
};

const buildCursors = (
  rows: CreatePipelineModalResourceRow[],
  readMode: ReadMode,
): PipelineEdge["cursors"] => {
  if (readMode !== ReadMode.INCREMENTAL) return [];
  return rows
    .filter((row) => !!row.cursorField)
    .map((row) => ({
      resource: row.name,
      field: row.cursorField,
      lookbackSeconds: 0n,
    })) as PipelineEdge["cursors"];
};

const buildSinkEdges = ({
  sourceId,
  rows,
  sink,
  isCdc,
}: {
  sourceId: string;
  rows: CreatePipelineModalResourceRow[];
  sink: CreatePipelineModalSinkRow;
  isCdc: boolean;
}): PipelineEdge[] => {
  const selectable = rows.filter((row) => row.isSelectable);
  const selected = rows.filter((row) => row.isSelected);
  const readModes = [
    ...new Set(selected.map((row) => (isCdc ? ReadMode.UNSPECIFIED : row.readMode))),
  ];

  const isCollapsible =
    !selectable.length || (selected.length === selectable.length && readModes.length <= 1);

  if (isCollapsible) {
    const readMode = readModes[0] ?? (isCdc ? ReadMode.UNSPECIFIED : ReadMode.FULL);
    return [
      {
        fromNode: sourceId,
        resource: "",
        toNode: sink.connection.id,
        readMode,
        writeMode: sink.writeMode,
        cursors: isCdc ? [] : buildCursors(selected, readMode),
        selector: "",
      } as PipelineEdge,
    ];
  }

  return selected.map((row) => {
    const readMode = isCdc ? ReadMode.UNSPECIFIED : row.readMode;
    return {
      fromNode: sourceId,
      resource: row.name,
      toNode: sink.connection.id,
      readMode,
      writeMode: sink.writeMode,
      cursors: isCdc ? [] : buildCursors([row], readMode),
      selector: "",
    } as PipelineEdge;
  });
};

const buildEdges = ({
  sourceConnection,
  rowsBySink,
  sinks,
  isCdc,
}: {
  sourceConnection: Connection | null;
  rowsBySink: Record<string, CreatePipelineModalResourceRow[]>;
  sinks: CreatePipelineModalSinkRow[];
  isCdc: boolean;
}): PipelineEdge[] => {
  if (!sourceConnection) return [];
  return sinks.flatMap((sink) =>
    buildSinkEdges({
      sourceId: sourceConnection.id,
      rows: rowsBySink[sink.connection.id] ?? [],
      sink,
      isCdc,
    }),
  );
};

export const mapCreatePipelineStateToVersionRequest = ({
  sourceConnection,
  rowsBySink,
  sinks,
  replication,
  pipelineId,
}: {
  sourceConnection: Connection | null;
  rowsBySink: Record<string, CreatePipelineModalResourceRow[]>;
  sinks: CreatePipelineModalSinkRow[];
  replication: ReplicationMode;
  pipelineId: string;
}): CreatePipelineVersionRequest =>
  create(CreatePipelineVersionRequestSchema, {
    pipelineId,
    nodes: buildNodes(sourceConnection, sinks),
    edges: buildEdges({
      sourceConnection,
      rowsBySink,
      sinks,
      isCdc: replication === ReplicationMode.CDC,
    }),
  });

export const mapCreatePipelineStateToRequest = (
  state: CreatePipelineModalState,
  name: string,
): CreatePipelineRequest =>
  create(CreatePipelineRequestSchema, {
    name: name.trim(),
    description: state.description.trim(),
    schedule: state.schedule.isEnabled
      ? {
          cron: mapPipelineScheduleStateToCron(state.schedule),
          timezone: state.schedule.timezone,
          enabled: true,
        }
      : undefined,
  });
