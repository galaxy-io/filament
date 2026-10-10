import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import { ExecutionMode, type WorkerConfiguration } from "@/gen/ingestion/v1/common_pb";

import { isPipelineRunnable } from "@/pages/pipelines/canvas/graph/diff";
import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";
import PipelinePageActiveRunActions from "@/pages/pipelines/components/header/PipelinePageActiveRunActions";
import PipelinePageRunButton from "@/pages/pipelines/components/header/PipelinePageRunButton";
import { usePipelineCanvasNavigate } from "@/pages/pipelines/hooks/usePipelineCanvasNavigate";
import { usePipelineRun } from "@/pages/pipelines/hooks/usePipelineRun";
import { getPipelineValidationErrors } from "@/pages/pipelines/utils";

import { usePipelineParams } from "@/module/hooks";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import { useSuspenseListActivePipelineRunsQuery } from "@/api/queries/runs";

import { isContinuousRunActive } from "@/utils/runs";

const PipelinePageRunActions: FC = () => {
  const { id } = usePipelineParams();
  const navigateCanvas = usePipelineCanvasNavigate();
  const { startRun, isRunning } = usePipelineRun();

  const { data } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const pipeline = data.pipeline;
  const currentVersion = pipeline?.currentVersion;
  const isContinuous = pipeline?.executionMode === ExecutionMode.CONTINUOUS;

  const { data: activeRunsData } = useSuspenseListActivePipelineRunsQuery(id);
  const activeRun = activeRunsData.runs.find((run) => !isContinuous || isContinuousRunActive(run));

  const validateInput = useMemo(
    () =>
      create(ValidatePipelineRequestSchema, {
        graph: currentVersion?.graph,
        executionMode: pipeline?.executionMode,
      }),
    [currentVersion, pipeline?.executionMode],
  );
  const {
    data: validation,
    isFetching: isValidating,
    error: validationError,
  } = useValidatePipelineQuery({ input: validateInput });
  const runErrors = useMemo(
    () => (validationError ? [validationError.message] : getPipelineValidationErrors(validation)),
    [validation, validationError],
  );

  if (!pipeline) {
    return null;
  }

  if (activeRun) {
    return <PipelinePageActiveRunActions run={activeRun} />;
  }

  const handleRun = (workerConfiguration?: WorkerConfiguration) => {
    startRun(pipeline, {
      workerConfiguration,
      onSuccess: () =>
        navigateCanvas({
          node: undefined,
          resource: undefined,
          showPanel: true,
          tab: PipelineCanvasPanelTab.ACTIVITY,
        }),
    });
  };

  return (
    <PipelinePageRunButton
      workerConfiguration={pipeline.workerConfiguration}
      runErrors={runErrors}
      isRunnable={isPipelineRunnable(currentVersion)}
      isRunning={isRunning || isValidating}
      onRun={handleRun}
    />
  );
};

export default PipelinePageRunActions;
