import { PipelineGroup } from "@/pages/pipelines/types";

// Layout constants
export const PIPELINE_CARD_HEIGHT = 48;
export const PIPELINE_GROUP_BAND_HEIGHT = 40;
export const PIPELINE_INDICATOR_WIDTH = 16;
export const PIPELINE_SEARCH_WIDTH = 280;
export const PIPELINE_MAX_VISIBLE_SINKS = 2;

// Maps
export const PIPELINE_GROUP_TO_LABEL_MAP: Record<PipelineGroup, string> = {
  [PipelineGroup.ACTIVE]: "Active",
  [PipelineGroup.NEEDS_ATTENTION]: "Needs attention",
  [PipelineGroup.PAUSED]: "Paused",
};

export const PIPELINE_METRIC_COLUMN_WIDTH_MAP = {
  connectors: 160,
  lastRun: 120,
  volume: 120,
  schedule: 120,
} as const;
