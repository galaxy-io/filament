import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";

import PipelinePageActiveRunActions from "@/pages/pipelines/components/header/PipelinePageActiveRunActions";
import PipelinePageRunButton from "@/pages/pipelines/components/header/PipelinePageRunButton";
import PipelinePageStreamButton from "@/pages/pipelines/components/header/PipelinePageStreamButton";
import { usePipelineExecutionLane } from "@/pages/pipelines/hooks/usePipelineExecutionLane";
import { usePipelineRun } from "@/pages/pipelines/hooks/usePipelineRun";
import { getPipelineValidationErrors, isPipelineRunnable } from "@/pages/pipelines/utils";

import { useFilamentFlowOpen, usePipelineParams } from "@/module/hooks";
import { Flow } from "@/module/types";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import { useSuspenseListActivePipelineRunsQuery } from "@/api/queries/runs";

import { getErrorMessage } from "@/utils/errors";
import { isContinuousRun, isContinuousRunActive } from "@/utils/runs";

const PipelinePageRunActions: FC = () => {
  const { id } = usePipelineParams();
  const openFlow = useFilamentFlowOpen();

  const { data } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const pipeline = data.pipeline;
  const currentVersion = pipeline?.currentVersion;
  const isContinuous = pipeline?.executionMode === ExecutionMode.CONTINUOUS;

  const { startRun, isRunning } = usePipelineRun();
  const lane = usePipelineExecutionLane();

  const { data: activeRunsData } = useSuspenseListActivePipelineRunsQuery(id);
  const activeRun = activeRunsData.runs.find((run) => !isContinuous || isContinuousRunActive(run));
  const activeStream = activeRunsData.runs.find(isContinuousRunActive);
  const activeBatch = activeRunsData.runs.find((run) => !isContinuousRun(run));

  const validateInput = useMemo(
    () =>
      create(ValidatePipelineRequestSchema, {
        graph: currentVersion?.graph,
        executionMode: pipeline?.executionMode,
      }),
    [currentVersion, pipeline?.executionMode],
  );
  const { data: validation, error: validationError } = useValidatePipelineQuery({
    input: validateInput,
  });
  const runErrors = useMemo(
    () =>
      validationError
        ? [getErrorMessage(validationError, "Failed to validate pipeline")]
        : getPipelineValidationErrors(validation),
    [validation, validationError],
  );

  if (!pipeline) {
    return null;
  }

  if (lane.isSplit && lane.isStreamLane) {
    if (activeStream) return <PipelinePageActiveRunActions run={activeStream} isStream />;
    return (
      <PipelinePageStreamButton
        runErrors={runErrors}
        isRunnable={lane.hasStreamResources}
        isStarting={isRunning}
        onClick={() => startRun(pipeline, { executionMode: ExecutionMode.CONTINUOUS })}
      />
    );
  }

  const laneRun = lane.isSplit ? activeBatch : activeRun;
  if (laneRun) {
    return <PipelinePageActiveRunActions run={laneRun} />;
  }

  return (
    <PipelinePageRunButton
      runErrors={runErrors}
      isRunnable={isPipelineRunnable(currentVersion)}
      onClick={() => openFlow(Flow.RUN_PIPELINE, { pipelineId: pipeline.id })}
    />
  );
};

export default PipelinePageRunActions;
