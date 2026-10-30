import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { OBSERVABILITY_RUN_STATUSES } from "@/pages/observability/components/runs/constants";

export const OBSERVABILITY_METRICS_FEATURED_STATUSES = [
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.RUNNING,
];

export const OBSERVABILITY_METRICS_OTHER_STATUSES = OBSERVABILITY_RUN_STATUSES.filter(
  (status) =>
    !OBSERVABILITY_METRICS_FEATURED_STATUSES.includes(status) && status !== RunStatus.SCHEDULED,
);
