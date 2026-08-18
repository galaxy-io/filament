import type { Theme } from "@galaxy-io/dls/theme/types";

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

export const PIPELINES_TABLE_RECENT_RUNS_STATUS_TO_COLOR_MAP: Record<
  RunStatus,
  (theme: Theme) => string
> = {
  [RunStatus.UNSPECIFIED]: (theme) => theme.color.border.primary,
  [RunStatus.REQUESTED]: (theme) => theme.color.icon.lime,
  [RunStatus.RUNNING]: (theme) => theme.color.icon.blue,
  [RunStatus.COMPLETED]: (theme) => theme.color.icon.success,
  [RunStatus.FAILED]: (theme) => theme.color.icon.error,
  [RunStatus.CANCELED]: (theme) => theme.color.icon.purple,
  [RunStatus.PAUSED]: (theme) => theme.color.icon.warning,
  [RunStatus.PARTIAL]: (theme) => theme.color.icon.pink,
  [RunStatus.SCHEDULED]: (theme) => theme.color.icon.yellow,
};
