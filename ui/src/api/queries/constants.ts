import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

export const PROBE_QUERY_OPTIONS = {
  retry: false,
  networkMode: "always",
  staleTime: Number.POSITIVE_INFINITY,
  refetchOnWindowFocus: false,
} as const;

export const ACTIVE_RUN_STATUSES = new Set<RunStatus>([
  RunStatus.REQUESTED,
  RunStatus.RUNNING,
  RunStatus.PAUSED,
]);
