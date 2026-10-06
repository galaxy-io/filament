import { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import { ChartPalette } from "@galaxy-io/dls/charts/types";

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

export const PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP: Record<RunStatus, BeaconVariant> = {
  [RunStatus.UNSPECIFIED]: BeaconVariant.TERTIARY,
  [RunStatus.REQUESTED]: BeaconVariant.SECONDARY,
  [RunStatus.RUNNING]: BeaconVariant.PRIMARY,
  [RunStatus.COMPLETED]: BeaconVariant.SUCCESS,
  [RunStatus.FAILED]: BeaconVariant.ERROR,
  [RunStatus.CANCELED]: BeaconVariant.TERTIARY,
  [RunStatus.PAUSED]: BeaconVariant.SECONDARY,
  [RunStatus.PARTIAL]: BeaconVariant.WARNING,
  [RunStatus.SCHEDULED]: BeaconVariant.TERTIARY,
};

export const PIPELINE_RUN_STATUS_PULSING = new Set<RunStatus>([
  RunStatus.REQUESTED,
  RunStatus.RUNNING,
]);

export const PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP: Record<RunStatus, ChartPalette> = {
  [RunStatus.UNSPECIFIED]: ChartPalette.TERTIARY,
  [RunStatus.REQUESTED]: ChartPalette.SECONDARY,
  [RunStatus.RUNNING]: ChartPalette.PRIMARY,
  [RunStatus.COMPLETED]: ChartPalette.SUCCESS,
  [RunStatus.FAILED]: ChartPalette.ERROR,
  [RunStatus.CANCELED]: ChartPalette.TERTIARY,
  [RunStatus.PAUSED]: ChartPalette.SECONDARY,
  [RunStatus.PARTIAL]: ChartPalette.WARNING,
  [RunStatus.SCHEDULED]: ChartPalette.TERTIARY,
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

export const PIPELINE_EXECUTION_OBSERVED_STATE_TO_BEACON_VARIANT_MAP: Record<
  ExecutionObservedState,
  BeaconVariant
> = {
  [ExecutionObservedState.UNSPECIFIED]: BeaconVariant.TERTIARY,
  [ExecutionObservedState.STARTING]: BeaconVariant.PRIMARY,
  [ExecutionObservedState.RUNNING]: BeaconVariant.PRIMARY,
  [ExecutionObservedState.DRAINING]: BeaconVariant.PRIMARY,
  [ExecutionObservedState.PAUSED]: BeaconVariant.SECONDARY,
  [ExecutionObservedState.STOPPED]: BeaconVariant.TERTIARY,
  [ExecutionObservedState.RETRYING]: BeaconVariant.WARNING,
  [ExecutionObservedState.BLOCKED]: BeaconVariant.ERROR,
  [ExecutionObservedState.FAILED]: BeaconVariant.ERROR,
};

export const PIPELINE_EXECUTION_OBSERVED_STATE_PULSING = new Set<ExecutionObservedState>([
  ExecutionObservedState.STARTING,
  ExecutionObservedState.RUNNING,
  ExecutionObservedState.DRAINING,
  ExecutionObservedState.RETRYING,
]);
