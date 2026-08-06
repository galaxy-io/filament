import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";

import { IngestionType, type ReplicationMode } from "@/gen/ingestion/v1/common_pb";
import type { ConnectorSpec, ResourceColumn } from "@/gen/ingestion/v1/providers_pb";

import { INGESTION_TYPE_TO_REPLICATION_MODE_MAP } from "@/pages/pipelines/canvas/constants";
import { INGESTION_TYPE_TO_LABEL_MAP } from "@/pages/pipelines/canvas/edges/constants";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { getPipelineCanvasEdgeData } from "@/pages/pipelines/canvas/utils";

const UNRANKED_COLUMN_RANK = Number.MAX_SAFE_INTEGER;

export const PIPELINE_CANVAS_INGESTION_TYPES = [
  IngestionType.SNAPSHOT_REPLACE,
  IngestionType.SNAPSHOT_UPSERT,
  IngestionType.APPEND,
  IngestionType.UPSERT,
  IngestionType.CDC,
];

const getSpecReplicationModes = (spec: ConnectorSpec | undefined): Set<ReplicationMode> =>
  new Set((spec?.capabilities?.sourcePolicies ?? []).map((policy) => policy.mode));

export const getIngestionTypeOptions = (
  spec: ConnectorSpec | undefined,
  ingestionType: IngestionType,
): SelectInputOption[] => {
  const modes = getSpecReplicationModes(spec);

  return PIPELINE_CANVAS_INGESTION_TYPES.filter(
    (candidate) =>
      candidate === ingestionType ||
      modes.size === 0 ||
      modes.has(INGESTION_TYPE_TO_REPLICATION_MODE_MAP[candidate]),
  ).map((candidate) => ({
    id: String(candidate),
    label: INGESTION_TYPE_TO_LABEL_MAP[candidate],
    value: candidate,
  }));
};

export const isColumnSelectable = (column: ResourceColumn): boolean =>
  column.cursorEligible && column.configurable;

const getColumnRank = (column: ResourceColumn): number =>
  column.recommendationRank || UNRANKED_COLUMN_RANK;

export const sortCursorColumns = (columns: ResourceColumn[]): ResourceColumn[] =>
  [...columns].sort((a, b) => {
    if (isColumnSelectable(a) !== isColumnSelectable(b)) return isColumnSelectable(a) ? -1 : 1;
    if (a.cursorRecommended !== b.cursorRecommended) return a.cursorRecommended ? -1 : 1;
    if (getColumnRank(a) !== getColumnRank(b)) return getColumnRank(a) - getColumnRank(b);
    return a.name.localeCompare(b.name);
  });

export const formatColumnType = (column: ResourceColumn): string => {
  const type = column.nativeType || column.logicalType;
  return column.nullable ? `${type} · nullable` : type;
};

export const getEdgeCursor = (edge: CanvasEdge, resource: string) =>
  getPipelineCanvasEdgeData(edge).cursors.find((cursor) => cursor.resource === resource) ?? null;
