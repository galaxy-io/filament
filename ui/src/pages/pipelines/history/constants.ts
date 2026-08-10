import { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import { TextVariant } from "@galaxy-io/dls/text/Text";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const PIPELINE_RUN_HISTORY_LIMIT = 50;
export const PIPELINE_RUN_RESOURCE_LOADING_ROW_COUNT = 3;
export const PIPELINE_RUN_ERROR_TOOLTIP_MAX_WIDTH = 360;

export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STATUS = 110;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VERSION = 110;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_RECORDS = 90;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_VOLUME = 100;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_STARTED_AT = 140;
export const PIPELINE_HISTORY_RUN_TABLE_COLUMN_WIDTH_DURATION = 120;

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
