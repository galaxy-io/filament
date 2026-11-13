import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINE_RUN_STATUS_TO_HUE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/components/runs/constants";

export const isPipelineRunCompletedWithFailures = (
  status: RunStatus,
  failedResources: RunInfo["failedResources"] = [],
): boolean => status === RunStatus.COMPLETED && failedResources.length > 0;

export const getPipelineRunStatusLabel = (
  status: RunStatus,
  failedResources?: RunInfo["failedResources"],
): string =>
  isPipelineRunCompletedWithFailures(status, failedResources)
    ? PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.PARTIAL]
    : PIPELINE_RUN_STATUS_TO_LABEL_MAP[status];

export const getPipelineRunStatusHue = (
  status: RunStatus,
  failedResources?: RunInfo["failedResources"],
): RoleColor | undefined =>
  isPipelineRunCompletedWithFailures(status, failedResources)
    ? PIPELINE_RUN_STATUS_TO_HUE_MAP[RunStatus.PARTIAL]
    : PIPELINE_RUN_STATUS_TO_HUE_MAP[status];

export const formatPipelineRunFailedResources = (
  failedResources: RunInfo["failedResources"],
): string => `Failed resources: ${failedResources.join(", ")}`;
