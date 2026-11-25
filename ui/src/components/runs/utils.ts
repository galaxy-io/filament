import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import { ExecutionObservedState, type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_HUE_MAP,
  PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP,
  PIPELINE_RUN_STATUS_TO_HUE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/components/runs/constants";

import { ACTIVE_RUN_STATUSES } from "@/constants";

export const isPipelineRunCompletedWithFailures = (
  status: RunStatus,
  failedResources: RunInfo["failedResources"] = [],
): boolean => status === RunStatus.COMPLETED && failedResources.length > 0;

const getPipelineRunObservedState = (
  status: RunStatus,
  executionStatus?: RunInfo["executionStatus"],
): ExecutionObservedState | undefined =>
  ACTIVE_RUN_STATUSES.has(status) ? executionStatus?.observedState : undefined;

export const isPipelineRunBlocked = ({ status, executionStatus }: RunInfo): boolean =>
  getPipelineRunObservedState(status, executionStatus) === ExecutionObservedState.BLOCKED;

export const getPipelineRunStatusLabel = (
  status: RunStatus,
  failedResources?: RunInfo["failedResources"],
  executionStatus?: RunInfo["executionStatus"],
): string => {
  const observedState = getPipelineRunObservedState(status, executionStatus);
  if (observedState !== undefined) {
    return PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP[observedState];
  }
  return isPipelineRunCompletedWithFailures(status, failedResources)
    ? PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.PARTIAL]
    : PIPELINE_RUN_STATUS_TO_LABEL_MAP[status];
};

export const getPipelineRunStatusHue = (
  status: RunStatus,
  failedResources?: RunInfo["failedResources"],
  executionStatus?: RunInfo["executionStatus"],
): RoleColor | undefined => {
  const observedState = getPipelineRunObservedState(status, executionStatus);
  if (observedState !== undefined) {
    return PIPELINE_EXECUTION_OBSERVED_STATE_TO_HUE_MAP[observedState];
  }
  return isPipelineRunCompletedWithFailures(status, failedResources)
    ? PIPELINE_RUN_STATUS_TO_HUE_MAP[RunStatus.PARTIAL]
    : PIPELINE_RUN_STATUS_TO_HUE_MAP[status];
};

export const formatPipelineRunFailedResources = (
  failedResources: RunInfo["failedResources"],
): string => `Failed resources: ${failedResources.join(", ")}`;
