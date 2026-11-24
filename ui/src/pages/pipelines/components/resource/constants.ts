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

export const PIPELINE_RESOURCE_SINK_FIELD_UNSET_ID = "__unset__";

export const PIPELINE_RESOURCE_PARTITION_BY_FIELD = "partition_by";

export const PIPELINE_RESOURCE_PARTITION_IDENTITY = "identity";

const PARTITION_KEYED_TRANSFORMS: string[] = [PIPELINE_RESOURCE_PARTITION_IDENTITY, "bucket[16]"];

export const LOGICAL_TYPE_TO_PARTITION_TRANSFORMS_MAP: Record<string, string[]> = {
  bool: [PIPELINE_RESOURCE_PARTITION_IDENTITY],
  int16: PARTITION_KEYED_TRANSFORMS,
  int32: PARTITION_KEYED_TRANSFORMS,
  int64: PARTITION_KEYED_TRANSFORMS,
  decimal: PARTITION_KEYED_TRANSFORMS,
  string: PARTITION_KEYED_TRANSFORMS,
  uuid: PARTITION_KEYED_TRANSFORMS,
  date: [PIPELINE_RESOURCE_PARTITION_IDENTITY, "day", "month", "year"],
  timestamp: ["hour", "day", "month", "year"],
  timestamptz: ["hour", "day", "month", "year"],
};
