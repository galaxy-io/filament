import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import {
  ExecutionDesiredState,
  ExecutionObservedState,
  type RunInfo,
  RunSignal,
  RunStatus,
} from "@/gen/ingestion/v1/runs_pb";

export const isContinuousRun = (run: RunInfo) => run.executionMode === ExecutionMode.CONTINUOUS;

// Desired stop may precede confirmed shutdown. Keep polling and expose the
// draining/blocked worker until the observed state confirms it has stopped.
export const isContinuousRunActive = (run: RunInfo) =>
  isContinuousRun(run) &&
  (!run.executionStatus || run.executionStatus.observedState !== ExecutionObservedState.STOPPED);

export const runPauseSignal = (run: RunInfo) =>
  (
    isContinuousRun(run)
      ? run.executionStatus?.desiredState === ExecutionDesiredState.PAUSED
      : run.status === RunStatus.PAUSED
  )
    ? RunSignal.RESUME
    : RunSignal.PAUSE;

export const runStopSignal = (run: RunInfo) =>
  isContinuousRun(run) ? RunSignal.STOP : RunSignal.CANCEL;

export const executionStateLabels: Record<ExecutionObservedState, string> = {
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

export const executionStateStatus: Record<ExecutionObservedState, RunStatus> = {
  [ExecutionObservedState.UNSPECIFIED]: RunStatus.UNSPECIFIED,
  [ExecutionObservedState.STARTING]: RunStatus.REQUESTED,
  [ExecutionObservedState.RUNNING]: RunStatus.RUNNING,
  [ExecutionObservedState.DRAINING]: RunStatus.REQUESTED,
  [ExecutionObservedState.PAUSED]: RunStatus.PAUSED,
  [ExecutionObservedState.STOPPED]: RunStatus.CANCELED,
  [ExecutionObservedState.RETRYING]: RunStatus.REQUESTED,
  [ExecutionObservedState.BLOCKED]: RunStatus.FAILED,
  [ExecutionObservedState.FAILED]: RunStatus.FAILED,
};

export const parseStreamResources = (text: string): string[] => [
  ...new Set(
    text
      .split(/\r?\n/)
      .map((name) => name.trim())
      .filter(Boolean),
  ),
];

// A pipeline needs one execution mode supported by every route.
export const commonExecutionModes = (
  routes: { supportedExecutionModes: ExecutionMode[] }[],
): ExecutionMode[] | undefined =>
  routes.length
    ? [ExecutionMode.BOUNDED, ExecutionMode.CONTINUOUS].filter((mode) =>
        routes.every((route) => route.supportedExecutionModes.includes(mode)),
      )
    : undefined;

export const streamResourceLabel = (subject: string) =>
  subject.replace(/[^a-zA-Z0-9_]+/g, "_").replace(/^_+|_+$/g, "");
