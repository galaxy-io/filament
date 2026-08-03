import { IngestionType } from "@/gen/ingestion/v1/common_pb";

export const INGESTION_TYPE_TO_LABEL_MAP: Record<IngestionType, string> = {
  [IngestionType.UNSPECIFIED]: "Snapshot + Replace",
  [IngestionType.SNAPSHOT_REPLACE]: "Snapshot + Replace",
  [IngestionType.SNAPSHOT_UPSERT]: "Snapshot + Upsert",
  [IngestionType.APPEND]: "Append",
  [IngestionType.UPSERT]: "Upsert",
  [IngestionType.DELETE]: "Delete",
  [IngestionType.CDC]: "CDC",
};

export const PIPELINE_CANVAS_EDGE_MODE_SELECT_WIDTH = 180;
