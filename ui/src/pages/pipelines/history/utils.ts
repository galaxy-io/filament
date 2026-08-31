import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const getPipelineHistoryRunTimestamp = (
  run: RunInfo,
): { timestamp: bigint; isStarted: boolean } => {
  if (run.startedAt) return { timestamp: run.startedAt, isStarted: true };
  if (run.status === RunStatus.SCHEDULED) return { timestamp: run.scheduledAt, isStarted: false };
  return { timestamp: run.requestedAt, isStarted: false };
};
