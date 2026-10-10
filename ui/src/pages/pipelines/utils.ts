import type {
  EdgeValidation,
  Requirement,
  ValidatePipelineResponse,
} from "@/gen/ingestion/v1/capabilities_pb";
import type { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  ExecutionDesiredState,
  type RunInfo,
  RunSignal,
  RunStatus,
} from "@/gen/ingestion/v1/runs_pb";

import { PIPELINE_EXECUTION_MODES } from "@/pages/pipelines/constants";

import { isContinuousRun } from "@/utils/runs";

export const getSupportedExecutionModes = (source: Connection | null): ExecutionMode[] =>
  source ? PIPELINE_EXECUTION_MODES.filter((mode) => source.executionModes.includes(mode)) : [];

export const getRunPauseSignal = (run: RunInfo) => {
  const isPaused = isContinuousRun(run)
    ? run.executionStatus?.desiredState === ExecutionDesiredState.PAUSED
    : run.status === RunStatus.PAUSED;
  return isPaused ? RunSignal.RESUME : RunSignal.PAUSE;
};

export const getRunStopSignal = (run: RunInfo) =>
  isContinuousRun(run) ? RunSignal.STOP : RunSignal.CANCEL;

export const getEdgeBlockingRequirements = (edge: EdgeValidation): Requirement[] =>
  [...edge.requirements, ...edge.resources.flatMap((resource) => resource.requirements)].filter(
    (requirement) => requirement.blocking,
  );

export const getEdgeValidationErrors = (edge: EdgeValidation): string[] => [
  ...edge.errors.map((error) => error.message),
  ...getEdgeBlockingRequirements(edge).map((requirement) => requirement.message),
];

export const getPipelineValidationErrors = (
  validation: ValidatePipelineResponse | undefined,
): string[] => [
  ...new Set([
    ...(validation?.errors ?? []).map((error) => error.message),
    ...(validation?.edges ?? []).flatMap(getEdgeValidationErrors),
  ]),
];
