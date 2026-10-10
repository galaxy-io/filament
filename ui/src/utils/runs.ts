import { EMPTY_VALUE, formatDuration } from "@galaxy-io/dls/utils/format";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import { ExecutionObservedState, type RunInfo } from "@/gen/ingestion/v1/runs_pb";

import { ACTIVE_RUN_STATUSES } from "@/constants";

export const isContinuousRun = (run: RunInfo) => run.executionMode === ExecutionMode.CONTINUOUS;

export const isContinuousRunActive = (run: RunInfo) =>
  isContinuousRun(run) && run.executionStatus?.observedState !== ExecutionObservedState.STOPPED;

export const isRunActive = (run: RunInfo) =>
  ACTIVE_RUN_STATUSES.has(run.status) || isContinuousRunActive(run);

export const formatRunDuration = (startedAt: bigint, endedAt: bigint) =>
  startedAt && endedAt ? formatDuration(endedAt - startedAt) : EMPTY_VALUE;
