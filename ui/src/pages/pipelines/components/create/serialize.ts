import { create } from "@bufbuild/protobuf";

import { ConnectorKind, ReadMode, ReplicationMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type CreatePipelineRequest,
  CreatePipelineRequestSchema,
  type CreatePipelineVersionRequest,
  CreatePipelineVersionRequestSchema,
  type Pipeline,
  type PipelineEdge,
  PipelineEdgeSchema,
  type PipelineNode,
  PipelineNodeSchema,
  type ResourceCursorConfig,
  ResourceCursorConfigSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import type {
  CreatePipelineModalResourceRow,
  CreatePipelineModalSinkRow,
  CreatePipelineModalState,
} from "@/pages/pipelines/components/create/types";
import { mapPipelineScheduleStateToCron } from "@/pages/pipelines/settings/utils";

const buildNodes = (
  sourceConnection: Connection | null,
  sinks: CreatePipelineModalSinkRow[],
): PipelineNode[] => {
  if (!sourceConnection) return [];
  return [
    create(PipelineNodeSchema, {
      id: sourceConnection.id,
      kind: ConnectorKind.SOURCE,
      connectionId: sourceConnection.id,
    }),
    ...sinks.map((sink) =>
      create(PipelineNodeSchema, {
        id: sink.connection.id,
        kind: ConnectorKind.SINK,
        connectionId: sink.connection.id,
      }),
    ),
  ];
};

const buildCursors = (
  rows: CreatePipelineModalResourceRow[],
  readMode: ReadMode,
): ResourceCursorConfig[] => {
  if (readMode !== ReadMode.INCREMENTAL) return [];
  return rows
    .filter((row) => !!row.cursorField)
    .map((row) =>
      create(ResourceCursorConfigSchema, {
        resource: row.name,
        field: row.cursorField,
        lookbackSeconds: 0n,
      }),
    );
};

const buildSinkEdges = ({
  sourceId,
  rows,
  sink,
  isCdc,
}: {
  sourceId: PipelineNode["id"];
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
      create(PipelineEdgeSchema, {
        fromNode: sourceId,
        resource: "",
        toNode: sink.connection.id,
        readMode,
        writeMode: sink.writeMode,
        cursors: isCdc ? [] : buildCursors(selected, readMode),
      }),
    ];
  }

  return selected.map((row) => {
    const readMode = isCdc ? ReadMode.UNSPECIFIED : row.readMode;
    return create(PipelineEdgeSchema, {
      fromNode: sourceId,
      resource: row.name,
      toNode: sink.connection.id,
      readMode,
      writeMode: sink.writeMode,
      cursors: isCdc ? [] : buildCursors([row], readMode),
    });
  });
};

const buildEdges = ({
  sourceConnection,
  rowsBySink,
  sinks,
  isCdc,
}: {
  sourceConnection: Connection | null;
  rowsBySink: Record<Connection["id"], CreatePipelineModalResourceRow[]>;
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
  rowsBySink: Record<Connection["id"], CreatePipelineModalResourceRow[]>;
  sinks: CreatePipelineModalSinkRow[];
  replication: ReplicationMode;
  pipelineId: Pipeline["id"];
}): CreatePipelineVersionRequest =>
  create(CreatePipelineVersionRequestSchema, {
    pipelineId,
    graph: {
      nodes: buildNodes(sourceConnection, sinks),
      edges: buildEdges({
        sourceConnection,
        rowsBySink,
        sinks,
        isCdc: replication === ReplicationMode.CDC,
      }),
    },
  });

export const mapCreatePipelineStateToRequest = (
  state: CreatePipelineModalState,
  name: Pipeline["name"],
): CreatePipelineRequest =>
  create(CreatePipelineRequestSchema, {
    name: name.trim(),
    description: state.description.trim(),
    schedule: state.schedule.isEnabled
      ? {
          cron: mapPipelineScheduleStateToCron(state.schedule),
          timezone: state.schedule.timezone,
          isEnabled: true,
        }
      : undefined,
  });
