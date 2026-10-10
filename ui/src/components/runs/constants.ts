import type { RoleColor } from "@galaxy-io/dls/theme/tokens/types";

import { ExecutionObservedState, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { getEnumValues } from "@/utils/select";

export const PIPELINE_RUN_DURATION_TICK_MS = 1_000;

export const PIPELINE_RUN_STATUS_TO_LABEL_MAP: Record<RunStatus, string> = {
  [RunStatus.UNSPECIFIED]: "Unknown",
  [RunStatus.REQUESTED]: "Requested",
  [RunStatus.RUNNING]: "Running",
  [RunStatus.COMPLETED]: "Completed",
  [RunStatus.FAILED]: "Failed",
  [RunStatus.CANCELED]: "Cancelled",
  [RunStatus.PAUSED]: "Paused",
  [RunStatus.PARTIAL]: "Partial",
  [RunStatus.SCHEDULED]: "Scheduled",
};

export const PIPELINE_RUN_STATUS_TO_HUE_MAP: Record<RunStatus, RoleColor | undefined> = {
  [RunStatus.UNSPECIFIED]: undefined,
  [RunStatus.REQUESTED]: "orange",
  [RunStatus.RUNNING]: "blue",
  [RunStatus.COMPLETED]: "success",
  [RunStatus.FAILED]: "error",
  [RunStatus.CANCELED]: undefined,
  [RunStatus.PAUSED]: "teal",
  [RunStatus.PARTIAL]: "pink",
  [RunStatus.SCHEDULED]: "yellow",
};

export const PIPELINE_EXECUTION_OBSERVED_STATE_TO_LABEL_MAP: Record<
  ExecutionObservedState,
  string
> = {
  [ExecutionObservedState.UNSPECIFIED]: "Unknown",
  [ExecutionObservedState.STARTING]: "Starting",
  [ExecutionObservedState.RUNNING]: "Running",
  [ExecutionObservedState.DRAINING]: "Draining",
  [ExecutionObservedState.PAUSED]: "Paused",
  [ExecutionObservedState.STOPPED]: "Stopped",
  [ExecutionObservedState.RETRYING]: "Retrying",
  [ExecutionObservedState.BLOCKED]: "Blocked",
  [ExecutionObservedState.FAILED]: "Failed",
};

export const PIPELINE_EXECUTION_OBSERVED_STATE_TO_HUE_MAP: Record<
  ExecutionObservedState,
  RoleColor | undefined
> = {
  [ExecutionObservedState.UNSPECIFIED]: undefined,
  [ExecutionObservedState.STARTING]: "orange",
  [ExecutionObservedState.RUNNING]: "blue",
  [ExecutionObservedState.DRAINING]: "orange",
  [ExecutionObservedState.PAUSED]: "teal",
  [ExecutionObservedState.STOPPED]: undefined,
  [ExecutionObservedState.RETRYING]: "yellow",
  [ExecutionObservedState.BLOCKED]: "error",
  [ExecutionObservedState.FAILED]: "error",
};

export const PIPELINE_RUN_EXECUTED_STATUSES = getEnumValues(RunStatus).filter(
  (status) => status !== RunStatus.SCHEDULED,
);
