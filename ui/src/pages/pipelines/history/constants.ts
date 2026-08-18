import { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import { ChartPalette } from "@galaxy-io/dls/charts/types";
import { TextVariant } from "@galaxy-io/dls/text/Text";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const PIPELINE_RUN_RESOURCE_LOADING_ROW_COUNT = 1;
export const PIPELINE_RUN_ERROR_TOOLTIP_MAX_WIDTH = 360;

export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS = 110;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION = 120;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS = 100;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME = 100;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION = 100;

export const PIPELINE_RUN_STATUS_TO_LABEL_MAP: Record<RunStatus, string> = {
  [RunStatus.UNSPECIFIED]: "Unknown",
  [RunStatus.REQUESTED]: "Requested",
  [RunStatus.RUNNING]: "Running",
  [RunStatus.COMPLETED]: "Completed",
  [RunStatus.FAILED]: "Failed",
  [RunStatus.CANCELED]: "Canceled",
  [RunStatus.PAUSED]: "Paused",
  [RunStatus.PARTIAL]: "Partial",
  [RunStatus.SCHEDULED]: "Scheduled",
};

export const PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP: Record<RunStatus, BeaconVariant> = {
  [RunStatus.UNSPECIFIED]: BeaconVariant.SECONDARY,
  [RunStatus.REQUESTED]: BeaconVariant.LIME,
  [RunStatus.RUNNING]: BeaconVariant.BLUE,
  [RunStatus.COMPLETED]: BeaconVariant.SUCCESS,
  [RunStatus.FAILED]: BeaconVariant.ERROR,
  [RunStatus.CANCELED]: BeaconVariant.PURPLE,
  [RunStatus.PAUSED]: BeaconVariant.WARNING,
  [RunStatus.PARTIAL]: BeaconVariant.PINK,
  [RunStatus.SCHEDULED]: BeaconVariant.YELLOW,
};

export const PIPELINE_RUN_STATUS_TO_CHART_PALETTE_MAP: Record<RunStatus, ChartPalette | undefined> =
  {
    [RunStatus.UNSPECIFIED]: undefined,
    [RunStatus.COMPLETED]: ChartPalette.GREEN,
    [RunStatus.FAILED]: ChartPalette.RED,
    [RunStatus.RUNNING]: ChartPalette.BLUE,
    [RunStatus.REQUESTED]: ChartPalette.LIME,
    [RunStatus.SCHEDULED]: ChartPalette.YELLOW,
    [RunStatus.CANCELED]: ChartPalette.PURPLE,
    [RunStatus.PAUSED]: ChartPalette.ORANGE,
    [RunStatus.PARTIAL]: ChartPalette.PINK,
  };

export const PIPELINE_RUN_STATUS_TO_TEXT_VARIANT_MAP: Record<RunStatus, TextVariant> = {
  [RunStatus.UNSPECIFIED]: TextVariant.SECONDARY,
  [RunStatus.REQUESTED]: TextVariant.LIME,
  [RunStatus.RUNNING]: TextVariant.BLUE,
  [RunStatus.COMPLETED]: TextVariant.SUCCESS,
  [RunStatus.FAILED]: TextVariant.ERROR,
  [RunStatus.CANCELED]: TextVariant.PURPLE,
  [RunStatus.PAUSED]: TextVariant.WARNING,
  [RunStatus.PARTIAL]: TextVariant.PINK,
  [RunStatus.SCHEDULED]: TextVariant.YELLOW,
};
