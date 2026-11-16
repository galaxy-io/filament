import { TextVariant } from "@galaxy-io/dls/text/Text";

import { ObservabilityPipelinesColumn } from "@/pages/observability/components/pipelines/types";
import { ObservabilityPipelineHealth } from "@/pages/observability/types";

export const OBSERVABILITY_PIPELINES_CONTAINER_NAME = "observability-pipelines";
export const OBSERVABILITY_PIPELINES_HEADER_HEIGHT = 32;
export const OBSERVABILITY_PIPELINES_ROW_HEIGHT = 44;
export const OBSERVABILITY_PIPELINES_FOOTER_HEIGHT = 44;
export const OBSERVABILITY_PIPELINES_LIMIT = 10;
export const OBSERVABILITY_PIPELINES_PAGE_MIN_HEIGHT =
  OBSERVABILITY_PIPELINES_LIMIT * OBSERVABILITY_PIPELINES_ROW_HEIGHT;
export const OBSERVABILITY_PIPELINES_BAR_WIDTH = 6;
export const OBSERVABILITY_PIPELINES_BAR_GAP = 4;
export const OBSERVABILITY_PIPELINES_BAR_HEIGHT = 24;
export const OBSERVABILITY_PIPELINES_BAR_RUNNING_HEIGHT = 4;
const OBSERVABILITY_PIPELINES_FLOW_WIDTH = 140;
const OBSERVABILITY_PIPELINES_LAST_RUN_WIDTH = 72;
const OBSERVABILITY_PIPELINES_RECORDS_WIDTH = 104;
export const OBSERVABILITY_PIPELINES_HIDE_RECORDS_MAX_WIDTH = 800;
export const OBSERVABILITY_PIPELINES_HIDE_LAST_RUN_MAX_WIDTH = 720;
export const OBSERVABILITY_PIPELINES_HIDE_FLOW_MAX_WIDTH = 620;

const OBSERVABILITY_PIPELINES_TRACK = "var(--observability-pipelines-track)";

export const OBSERVABILITY_PIPELINES_COLUMNS = `${OBSERVABILITY_PIPELINES_FLOW_WIDTH}px minmax(0, 1fr) ${OBSERVABILITY_PIPELINES_TRACK} ${OBSERVABILITY_PIPELINES_LAST_RUN_WIDTH}px ${OBSERVABILITY_PIPELINES_RECORDS_WIDTH}px`;
export const OBSERVABILITY_PIPELINES_COLUMNS_NO_RECORDS = `${OBSERVABILITY_PIPELINES_FLOW_WIDTH}px minmax(0, 1fr) ${OBSERVABILITY_PIPELINES_TRACK} ${OBSERVABILITY_PIPELINES_LAST_RUN_WIDTH}px`;
export const OBSERVABILITY_PIPELINES_COLUMNS_NO_LAST_RUN = `${OBSERVABILITY_PIPELINES_FLOW_WIDTH}px minmax(0, 1fr) ${OBSERVABILITY_PIPELINES_TRACK}`;
export const OBSERVABILITY_PIPELINES_COLUMNS_NAME_AND_TRACK = `minmax(0, 1fr) ${OBSERVABILITY_PIPELINES_TRACK}`;

export const OBSERVABILITY_PIPELINES_COLUMN_TO_LABEL_MAP: Record<
  ObservabilityPipelinesColumn,
  string
> = {
  [ObservabilityPipelinesColumn.FLOW]: "Flow",
  [ObservabilityPipelinesColumn.PIPELINE]: "Pipeline",
  [ObservabilityPipelinesColumn.RUNS]: "Runs",
  [ObservabilityPipelinesColumn.LAST_RUN]: "Last run",
  [ObservabilityPipelinesColumn.RECORDS]: "Records",
};

export const OBSERVABILITY_PIPELINES_COLUMN_TO_ALIGN_MAP: Record<
  ObservabilityPipelinesColumn,
  "left" | "right"
> = {
  [ObservabilityPipelinesColumn.FLOW]: "left",
  [ObservabilityPipelinesColumn.PIPELINE]: "left",
  [ObservabilityPipelinesColumn.RUNS]: "left",
  [ObservabilityPipelinesColumn.LAST_RUN]: "left",
  [ObservabilityPipelinesColumn.RECORDS]: "right",
};

export const OBSERVABILITY_PIPELINE_HEALTH_TO_LAST_RUN_VARIANT_MAP: Record<
  ObservabilityPipelineHealth,
  TextVariant
> = {
  [ObservabilityPipelineHealth.FAILING]: TextVariant.ERROR,
  [ObservabilityPipelineHealth.RUNNING]: TextVariant.TERTIARY,
  [ObservabilityPipelineHealth.HEALTHY]: TextVariant.TERTIARY,
};
