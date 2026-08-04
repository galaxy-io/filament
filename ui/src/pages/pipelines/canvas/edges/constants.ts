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

export const INGESTION_TYPE_TO_DESCRIPTION_MAP: Record<IngestionType, string> =
  {
    [IngestionType.UNSPECIFIED]: "This ingestion is not configured.",
    [IngestionType.SNAPSHOT_REPLACE]:
      "This ingestion type takes a snapshot of the data and replaces the existing data.",
    [IngestionType.SNAPSHOT_UPSERT]:
      "This ingestion type takes a snapshot of the data and upserts the existing data.",
    [IngestionType.APPEND]:
      "This ingestion type appends new data to the existing data.",
    [IngestionType.UPSERT]:
      "This ingestion type upserts new data to the existing data.",
    [IngestionType.DELETE]: "This ingestion type deletes existing data.",
    [IngestionType.CDC]: "This ingestion type captures changes to the data.",
  };

export const PIPELINE_CANVAS_EDGE_MODE_SELECT_WIDTH = 180;
export const PIPELINE_CANVAS_EDGE_TOOLTIP_MAX_WIDTH = 240;
