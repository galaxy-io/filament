import { IngestionType } from "@/gen/ingestion/v1/common_pb";

import { PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE } from "@/pages/pipelines/canvas/nodes/constants";

export const INGESTION_TYPE_TO_LABEL_MAP: Record<IngestionType, string> = {
  [IngestionType.UNSPECIFIED]: "Snapshot + Replace",
  [IngestionType.SNAPSHOT_REPLACE]: "Snapshot + Replace",
  [IngestionType.SNAPSHOT_UPSERT]: "Snapshot + Upsert",
  [IngestionType.APPEND]: "Append",
  [IngestionType.UPSERT]: "Upsert",
  [IngestionType.DELETE]: "Delete",
  [IngestionType.CDC]: "Change Data Capture",
};

// Display order for the ingestion select; which of these are actually offered
// comes from ValidatePipeline.
export const PIPELINE_CANVAS_EDGE_ALL_INGESTION_TYPES = [
  IngestionType.SNAPSHOT_REPLACE,
  IngestionType.SNAPSHOT_UPSERT,
  IngestionType.APPEND,
  IngestionType.UPSERT,
  IngestionType.DELETE,
  IngestionType.CDC,
];

export const PIPELINE_CANVAS_EDGE_STUB_LENGTH = PIPELINE_CANVAS_NODE_HANDLE_SLOT_SIZE;

export const PIPELINE_CANVAS_EDGE_SELECT_WIDTH = 180;
export const PIPELINE_CANVAS_EDGE_COLUMN_LIST_MAX_HEIGHT = 180;

export const PIPELINE_CANVAS_DEFAULT_LOOKBACK_SECONDS = 300;

export const PIPELINE_CANVAS_EDGE_AUTO_CURSOR_LABEL = "Auto";
