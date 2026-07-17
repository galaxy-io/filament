import { BeaconVariant } from "@galaxy-io/dls/beacons/Beacon";
import { TextVariant } from "@galaxy-io/dls/text/Text";

import { PipelineGroup } from "@/pages/pipelines/types";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const PIPELINE_CARD_HEIGHT = 48;
export const PIPELINE_GROUP_BAND_HEIGHT = 40;
export const PIPELINE_INDICATOR_WIDTH = 16;
export const PIPELINE_MAX_VISIBLE_SINKS = 2;

export const PIPELINE_GROUP_TO_LABEL_MAP: Record<PipelineGroup, string> = {
  [PipelineGroup.ACTIVE]: "Active",
  [PipelineGroup.NEEDS_ATTENTION]: "Needs attention",
  [PipelineGroup.PAUSED]: "Paused",
};

export const PIPELINE_METRIC_COLUMN_WIDTH_CONNECTORS = 180;
export const PIPELINE_METRIC_COLUMN_WIDTH_LAST_RUN = 120;
export const PIPELINE_METRIC_COLUMN_WIDTH_VOLUME = 120;
export const PIPELINE_METRIC_COLUMN_WIDTH_SCHEDULE = 120;

export const RUN_HISTORY_LIMIT = 50;
export const RUN_HISTORY_LOADING_ROW_COUNT = 10;
export const RUN_TABLE_COLUMN_WIDTH_STATUS = 110;
export const RUN_TABLE_COLUMN_WIDTH_RECORDS = 110;
export const RUN_TABLE_COLUMN_WIDTH_VOLUME = 110;

export const RUN_STATUS_TO_LABEL_MAP: Record<RunStatus, string> = {
  [RunStatus.UNSPECIFIED]: "Unknown",
  [RunStatus.REQUESTED]: "Requested",
  [RunStatus.RUNNING]: "Running",
  [RunStatus.COMPLETED]: "Completed",
  [RunStatus.FAILED]: "Failed",
  [RunStatus.CANCELED]: "Canceled",
  [RunStatus.PAUSED]: "Paused",
  [RunStatus.PARTIAL]: "Partial",
};

export const RUN_STATUS_TO_BEACON_VARIANT_MAP: Record<RunStatus, BeaconVariant> = {
  [RunStatus.UNSPECIFIED]: BeaconVariant.SECONDARY,
  [RunStatus.REQUESTED]: BeaconVariant.SECONDARY,
  [RunStatus.RUNNING]: BeaconVariant.BLUE,
  [RunStatus.COMPLETED]: BeaconVariant.SUCCESS,
  [RunStatus.FAILED]: BeaconVariant.ERROR,
  [RunStatus.CANCELED]: BeaconVariant.SECONDARY,
  [RunStatus.PAUSED]: BeaconVariant.WARNING,
  [RunStatus.PARTIAL]: BeaconVariant.WARNING,
};

export const RUN_STATUS_TO_TEXT_VARIANT_MAP: Record<RunStatus, TextVariant> = {
  [RunStatus.UNSPECIFIED]: TextVariant.SECONDARY,
  [RunStatus.REQUESTED]: TextVariant.SECONDARY,
  [RunStatus.RUNNING]: TextVariant.BLUE,
  [RunStatus.COMPLETED]: TextVariant.SUCCESS,
  [RunStatus.FAILED]: TextVariant.ERROR,
  [RunStatus.CANCELED]: TextVariant.SECONDARY,
  [RunStatus.PAUSED]: TextVariant.WARNING,
  [RunStatus.PARTIAL]: TextVariant.WARNING,
};
