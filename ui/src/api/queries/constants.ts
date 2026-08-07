import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const ACTIVE_RUN_STATUSES = new Set<RunStatus>([
  RunStatus.REQUESTED,
  RunStatus.RUNNING,
  RunStatus.PAUSED,
]);
