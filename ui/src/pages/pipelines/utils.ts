import type {
  EdgeValidation,
  Requirement,
  ValidatePipelineResponse,
} from "@/gen/ingestion/v1/capabilities_pb";
import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import {
  ExecutionDesiredState,
  ExecutionObservedState,
  type RunInfo,
  RunSignal,
  RunStatus,
} from "@/gen/ingestion/v1/runs_pb";

import { PIPELINE_EXECUTION_MODES } from "@/pages/pipelines/constants";

import { stripDeletedName } from "@/utils/format";

export const getSupportedExecutionModes = (
  source: Connection | null,
  sinks: Connection[],
): ExecutionMode[] =>
  source
    ? PIPELINE_EXECUTION_MODES.filter((mode) =>
        [source, ...sinks].every((connection) => connection.executionModes.includes(mode)),
      )
    : [];

export const isContinuousRun = (run: RunInfo) => run.executionMode === ExecutionMode.CONTINUOUS;

export const isContinuousRunActive = (run: RunInfo) =>
  isContinuousRun(run) && run.executionStatus?.observedState !== ExecutionObservedState.STOPPED;

export const getRunPauseSignal = (run: RunInfo) => {
  const isPaused = isContinuousRun(run)
    ? run.executionStatus?.desiredState === ExecutionDesiredState.PAUSED
    : run.status === RunStatus.PAUSED;
  return isPaused ? RunSignal.RESUME : RunSignal.PAUSE;
};

export const getRunStopSignal = (run: RunInfo) =>
  isContinuousRun(run) ? RunSignal.STOP : RunSignal.CANCEL;

const getBlockingRequirementMessages = (requirements: Requirement[]): string[] =>
  requirements
    .filter((requirement) => requirement.blocking)
    .map((requirement) => requirement.message);

export const getEdgeValidationErrors = (edge: EdgeValidation): string[] => [
  ...edge.errors.map((error) => error.message),
  ...getBlockingRequirementMessages(edge.requirements),
  ...edge.resources.flatMap((resource) => getBlockingRequirementMessages(resource.requirements)),
];

export const getPipelineValidationErrors = (
  validation: ValidatePipelineResponse | undefined,
): string[] => [
  ...new Set([
    ...(validation?.errors ?? []).map((error) => error.message),
    ...(validation?.edges ?? []).flatMap(getEdgeValidationErrors),
  ]),
];

export const formatPipelineName = (pipeline: Pipeline, includeDeleted = false): string => {
  const name = includeDeleted ? pipeline.name : stripDeletedName(pipeline.name);
  if (name) {
    return name.replace(/->/g, "→");
  }
  return pipeline.id;
};
