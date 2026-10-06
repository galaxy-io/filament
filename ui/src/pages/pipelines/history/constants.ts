import { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import { ChartPalette } from "@galaxy-io/dls/charts/types";
import { TextVariant } from "@galaxy-io/dls/text/Text";

import { ExecutionObservedState, RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const PIPELINE_RUN_RESOURCE_LOADING_ROW_COUNT = 1;

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
  [RunStatus.UNSPECIFIED]: BeaconVariant.SECONDARY,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.REQUESTED]: BeaconVariant.ORANGE,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.RUNNING]: BeaconVariant.BLUE,
  [RunStatus.COMPLETED]: BeaconVariant.SUCCESS,
  [RunStatus.FAILED]: BeaconVariant.ERROR,
  [RunStatus.CANCELED]: BeaconVariant.SECONDARY,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.PAUSED]: BeaconVariant.TEAL,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.PARTIAL]: BeaconVariant.PINK,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.SCHEDULED]: BeaconVariant.YELLOW,
};

export const PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP: Record<RunStatus, ChartPalette | undefined> =
  {
    [RunStatus.UNSPECIFIED]: undefined,
    [RunStatus.COMPLETED]: ChartPalette.GREEN,
    [RunStatus.FAILED]: ChartPalette.ERROR,
    [RunStatus.RUNNING]: ChartPalette.BLUE,
    [RunStatus.REQUESTED]: ChartPalette.ORANGE,
    [RunStatus.SCHEDULED]: ChartPalette.YELLOW,
    [RunStatus.CANCELED]: ChartPalette.SECONDARY,
    [RunStatus.PAUSED]: ChartPalette.TEAL,
    [RunStatus.PARTIAL]: ChartPalette.PINK,
  };

export const PIPELINE_RUN_STATUS_TO_TEXT_VARIANT_MAP: Record<RunStatus, TextVariant> = {
  [RunStatus.UNSPECIFIED]: TextVariant.SECONDARY,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.REQUESTED]: TextVariant.ORANGE,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.RUNNING]: TextVariant.BLUE,
  [RunStatus.COMPLETED]: TextVariant.SUCCESS,
  [RunStatus.FAILED]: TextVariant.ERROR,
  [RunStatus.CANCELED]: TextVariant.SECONDARY,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.PAUSED]: TextVariant.TEAL,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.PARTIAL]: TextVariant.PINK,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [RunStatus.SCHEDULED]: TextVariant.YELLOW,
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
  [ExecutionObservedState.UNSPECIFIED]: BeaconVariant.SECONDARY,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.STARTING]: BeaconVariant.ORANGE,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.RUNNING]: BeaconVariant.BLUE,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.DRAINING]: BeaconVariant.ORANGE,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.PAUSED]: BeaconVariant.TEAL,
  [ExecutionObservedState.STOPPED]: BeaconVariant.SECONDARY,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.RETRYING]: BeaconVariant.YELLOW,
  [ExecutionObservedState.BLOCKED]: BeaconVariant.ERROR,
  [ExecutionObservedState.FAILED]: BeaconVariant.ERROR,
};

export const PIPELINE_EXECUTION_OBSERVED_STATE_TO_TEXT_VARIANT_MAP: Record<
  ExecutionObservedState,
  TextVariant
> = {
  [ExecutionObservedState.UNSPECIFIED]: TextVariant.SECONDARY,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.STARTING]: TextVariant.ORANGE,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.RUNNING]: TextVariant.BLUE,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.DRAINING]: TextVariant.ORANGE,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.PAUSED]: TextVariant.TEAL,
  [ExecutionObservedState.STOPPED]: TextVariant.SECONDARY,
  // @dls-migrate shared-enums.color-other: `variant` is meaning only (`PRIMARY SECONDARY TERTIARY SUCCESS WARNING ERROR DISABLED INHERIT`): pick the meaning, or put the category on a component that takes `color`.
  [ExecutionObservedState.RETRYING]: TextVariant.YELLOW,
  [ExecutionObservedState.BLOCKED]: TextVariant.ERROR,
  [ExecutionObservedState.FAILED]: TextVariant.ERROR,
};

export const PIPELINE_EXECUTION_OBSERVED_STATE_PULSING = new Set<ExecutionObservedState>([
  ExecutionObservedState.STARTING,
  ExecutionObservedState.RUNNING,
  ExecutionObservedState.DRAINING,
]);
