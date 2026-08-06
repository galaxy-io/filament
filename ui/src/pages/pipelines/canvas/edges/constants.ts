import { IngestionType } from "@/gen/ingestion/v1/common_pb";

export const INGESTION_TYPE_TO_LABEL_MAP: Record<IngestionType, string> = {
  [IngestionType.UNSPECIFIED]: "Snapshot + Replace",
  [IngestionType.SNAPSHOT_REPLACE]: "Snapshot + Replace",
  [IngestionType.SNAPSHOT_UPSERT]: "Snapshot + Upsert",
  [IngestionType.APPEND]: "Append",
  [IngestionType.UPSERT]: "Upsert",
  [IngestionType.DELETE]: "Delete",
  [IngestionType.CDC]: "Change Data Capture",
};

export const INGESTION_TYPE_TO_DESCRIPTION_MAP: Record<IngestionType, string> = {
  [IngestionType.UNSPECIFIED]:
    "Reads the whole resource on every run and replaces the destination contents.",
  [IngestionType.SNAPSHOT_REPLACE]:
    "Reads the whole resource on every run and replaces the destination contents.",
  [IngestionType.SNAPSHOT_UPSERT]:
    "Reads the whole resource on every run and upserts rows by primary key.",
  [IngestionType.APPEND]: "Appends newly read rows without touching existing ones.",
  [IngestionType.UPSERT]:
    "Reads only rows changed since the last run, tracked by a cursor column, and upserts them by primary key.",
  [IngestionType.DELETE]:
    "Reads only rows changed since the last run, tracked by a cursor column, and deletes them.",
  [IngestionType.CDC]: "Streams inserts, updates, and deletes from the source's change log.",
};

export const PIPELINE_CANVAS_EDGE_SELECT_WIDTH = 180;
export const PIPELINE_CANVAS_EDGE_TOOLTIP_MAX_WIDTH = 240;
export const PIPELINE_CANVAS_EDGE_COLUMN_LIST_MAX_HEIGHT = 180;

export const PIPELINE_CANVAS_DEFAULT_LOOKBACK_SECONDS = 300;

export const PIPELINE_CANVAS_EDGE_AUTO_CURSOR_LABEL = "Auto";
