import { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import { ChartPalette } from "@galaxy-io/dls/charts/types";
import type { PaletteColor } from "@galaxy-io/dls/theme/tokens/types";

import { ExecutionObservedState, RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS = 120;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION = 120;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS = 110;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME = 120;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION = 110;

export const PIPELINE_RUN_STATUS_TO_LABEL_MAP: Record<RunStatus, string> = {
  [RunStatus.UNSPECIFIED]: "Unknown",
  [RunStatus.REQUESTED]: "Requested",
  [RunStatus.RUNNING]: "Running",
  [RunStatus.COMPLETED]: "Completed",
  [RunStatus.FAILED]: "Failed",
  [RunStatus.CANCELED]: "Cancelled",
  [RunStatus.PAUSED]: "Paused",
  [RunStatus.PARTIAL]: "Partial",
  [RunStatus.SCHEDULED]: "Scheduled",
};

export type PipelineRunStatusBeaconColor = { variant: BeaconVariant } | { color: PaletteColor };

export const PIPELINE_RUN_STATUS_TO_BEACON_COLOR_MAP: Record<
  RunStatus,
  PipelineRunStatusBeaconColor
> = {
  [RunStatus.UNSPECIFIED]: { variant: BeaconVariant.TERTIARY },
  [RunStatus.REQUESTED]: { color: "orange" },
  [RunStatus.RUNNING]: { color: "blue" },
  [RunStatus.COMPLETED]: { variant: BeaconVariant.SUCCESS },
  [RunStatus.FAILED]: { variant: BeaconVariant.ERROR },
  [RunStatus.CANCELED]: { variant: BeaconVariant.TERTIARY },
  [RunStatus.PAUSED]: { color: "teal" },
  [RunStatus.PARTIAL]: { variant: BeaconVariant.WARNING },
  [RunStatus.SCHEDULED]: { color: "yellow" },
};

export const PIPELINE_RUN_STATUS_PULSING = new Set<RunStatus>([
  RunStatus.REQUESTED,
  RunStatus.RUNNING,
]);

export const PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP: Record<RunStatus, ChartPalette | undefined> =
  {
    [RunStatus.UNSPECIFIED]: undefined,
    [RunStatus.REQUESTED]: ChartPalette.ORANGE,
    [RunStatus.RUNNING]: ChartPalette.BLUE,
    [RunStatus.COMPLETED]: ChartPalette.SUCCESS,
    [RunStatus.FAILED]: ChartPalette.ERROR,
    [RunStatus.CANCELED]: undefined,
    [RunStatus.PAUSED]: ChartPalette.TEAL,
    [RunStatus.PARTIAL]: ChartPalette.WARNING,
    [RunStatus.SCHEDULED]: ChartPalette.YELLOW,
  };

export const PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP: Record<
  ExecutionObservedState,
  string
> = {
  [ExecutionObservedState.UNSPECIFIED]: "Unknown",
  [ExecutionObservedState.STARTING]: "Starting",
  [ExecutionObservedState.RUNNING]: "Running",
  [ExecutionObservedState.DRAINING]: "Draining",
  [ExecutionObservedState.PAUSED]: "Paused",
  [ExecutionObservedState.STOPPED]: "Stopped",
  [ExecutionObservedState.RETRYING]: "Retrying",
  [ExecutionObservedState.BLOCKED]: "Blocked",
  [ExecutionObservedState.FAILED]: "Failed",
};

export const PIPELINE_EXECUTION_OBSERVED_STATE_TO_BEACON_COLOR_MAP: Record<
  ExecutionObservedState,
  PipelineRunStatusBeaconColor
> = {
  [ExecutionObservedState.UNSPECIFIED]: { variant: BeaconVariant.TERTIARY },
  [ExecutionObservedState.STARTING]: { color: "orange" },
  [ExecutionObservedState.RUNNING]: { color: "blue" },
  [ExecutionObservedState.DRAINING]: { color: "orange" },
  [ExecutionObservedState.PAUSED]: { color: "teal" },
  [ExecutionObservedState.STOPPED]: { variant: BeaconVariant.TERTIARY },
  [ExecutionObservedState.RETRYING]: { color: "yellow" },
  [ExecutionObservedState.BLOCKED]: { variant: BeaconVariant.ERROR },
  [ExecutionObservedState.FAILED]: { variant: BeaconVariant.ERROR },
};

export const PIPELINE_EXECUTION_OBSERVED_STATE_PULSING = new Set<ExecutionObservedState>([
  ExecutionObservedState.STARTING,
  ExecutionObservedState.RUNNING,
  ExecutionObservedState.DRAINING,
  ExecutionObservedState.RETRYING,
]);

export const PIPELINE_HISTORY_RUN_INFO_LOADING_WIDTH = 240;
