import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

export type ObservabilityRunMetric = "runs";

export interface ObservabilityMockRunInfo {
  runId: string;
  pipelineId: string;
  pipelineName: string;
  status: RunStatus;
  startedAt: bigint;
  endedAt: bigint;
  records: bigint;
  bytes: bigint;
  error: string;
}
