import type { Space } from "@galaxy-io/dls/theme/enums";

import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";

export const READ_MODE_TO_WRITE_MODES_MAP: Record<ReadMode, WriteMode[]> = {
  [ReadMode.UNSPECIFIED]: [WriteMode.APPEND, WriteMode.REPLACE, WriteMode.UPSERT],
  [ReadMode.FULL]: [WriteMode.APPEND, WriteMode.REPLACE, WriteMode.UPSERT],
  [ReadMode.INCREMENTAL]: [WriteMode.APPEND, WriteMode.UPSERT],
  [ReadMode.CDC]: [WriteMode.APPEND, WriteMode.MERGE],
};

export const PIPELINE_RESOURCE_COLUMNS_TABLE_INSET: Space = 4;
export const PIPELINE_RESOURCE_COLUMNS_TABLE_WIDTH_TYPE = 200;
