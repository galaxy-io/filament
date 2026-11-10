import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import type { TimeseriesPoint } from "@/gen/metrics/v1/metrics_pb";

export interface ObservabilityPipelinesCell {
  bucketStartMs: TimeseriesPoint["bucketStartMs"];
  status: RunStatus | undefined;
  completed: number;
  failed: number;
  summary: string;
}

export interface ObservabilityPipelinesBarSegment {
  status: RunStatus;
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
