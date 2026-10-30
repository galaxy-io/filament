import { SortBy } from "@/gen/ingestion/v1/sorting_pb";

export const PIPELINES_TABLE_COLUMN_MIN_WIDTH_PIPELINE = 280;
export const PIPELINES_TABLE_COLUMN_WIDTH_FLOW = 200;
export const PIPELINES_TABLE_COLUMN_WIDTH_RECENT_RUNS = 200;
export const PIPELINES_TABLE_COLUMN_WIDTH_STATUS = 140;
export const PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN = 110;
export const PIPELINES_TABLE_COLUMN_WIDTH_LAST_DURATION = 100;
export const PIPELINES_TABLE_COLUMN_WIDTH_LAST_RECORDS = 100;

export const PIPELINES_TABLE_RECENT_RUNS_COUNT = 10;

export const PIPELINES_TABLE_COLUMN_LAYOUT_STORAGE_KEY = "filament:pipelines-table:column-layout";

export const PIPELINES_TABLE_COLUMN_ID_PIPELINE = "pipeline";

export const PIPELINES_TABLE_SORT_BY_TO_COLUMN_ID_MAP: Record<SortBy, string | undefined> = {
  [SortBy.UNSPECIFIED]: undefined,
  [SortBy.NAME]: PIPELINES_TABLE_COLUMN_ID_PIPELINE,
  [SortBy.CREATED_AT]: undefined,
  [SortBy.UPDATED_AT]: undefined,
};
