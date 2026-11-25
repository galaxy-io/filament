import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";
import type { TimeseriesPoint } from "@/gen/metrics/v1/metrics_pb";

export interface ObservabilityPipelinesCell {
  bucketStartMs: TimeseriesPoint["bucketStartMs"];
  liveRun: RunInfo | undefined;
  completed: number;
  failed: number;
  summary: string;
}

export interface ObservabilityPipelinesBarSegment {
  key: string;
  hue: RoleColor | undefined;
  height: number;
}

export type ObservabilityPipelinesCounts = Map<Pipeline["id"], Map<string, number>>;

export enum ObservabilityPipelinesColumn {
  FLOW = "FLOW",
  PIPELINE = "PIPELINE",
  RUNS = "RUNS",
  LAST_RUN = "LAST_RUN",
  RECORDS = "RECORDS",
}
