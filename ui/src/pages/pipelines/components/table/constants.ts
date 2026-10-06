import { t } from "@galaxy-io/dls/theme/tokens/t";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const PIPELINES_TABLE_COLUMN_MIN_WIDTH_PIPELINE = 280;
export const PIPELINES_TABLE_COLUMN_WIDTH_FLOW = 200;
export const PIPELINES_TABLE_COLUMN_WIDTH_RECENT_RUNS = 200;
export const PIPELINES_TABLE_COLUMN_WIDTH_STATUS = 140;
export const PIPELINES_TABLE_COLUMN_WIDTH_LAST_RUN = 110;
export const PIPELINES_TABLE_COLUMN_WIDTH_LAST_DURATION = 100;
export const PIPELINES_TABLE_COLUMN_WIDTH_LAST_VOLUME = 100;

export const PIPELINES_TABLE_RECENT_RUNS_COUNT = 10;

export const PIPELINES_TABLE_RECENT_RUNS_STATUSES: RunStatus[] = [
  RunStatus.REQUESTED,
  RunStatus.RUNNING,
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.CANCELED,
  RunStatus.PAUSED,
  RunStatus.PARTIAL,
];

export const PIPELINES_TABLE_RECENT_RUNS_STATUS_TO_COLOR_MAP: Record<RunStatus, string> = {
  [RunStatus.UNSPECIFIED]: t.color.border.primary,
  [RunStatus.REQUESTED]: t.color.text.secondary,
  [RunStatus.RUNNING]: t.color.text.primary,
  [RunStatus.COMPLETED]: t.color.text.success,
  [RunStatus.FAILED]: t.color.text.error,
  [RunStatus.CANCELED]: t.color.text.tertiary,
  [RunStatus.PAUSED]: t.color.text.secondary,
  [RunStatus.PARTIAL]: t.color.text.warning,
  [RunStatus.SCHEDULED]: t.color.text.tertiary,
};
