import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import { PauseIcon, PlayIcon, StopIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { IconWeight } from "@galaxy-io/dls/icons/Icon";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  ExecutionDesiredState,
  ExecutionObservedState,
  type RunInfo,
  RunSignal,
  SignalRunRequestSchema,
} from "@/gen/ingestion/v1/runs_pb";

import PipelineRunStatus from "@/components/runs/PipelineRunStatus";

import { getRunPauseSignal, getRunStopSignal } from "@/pages/pipelines/utils";

import { useSignalRunMutation } from "@/api/queries/runs";

import { getErrorMessage } from "@/utils/errors";

interface PipelinePageActiveRunActionsProps {
  run: RunInfo;
}

const PipelinePageActiveRunActions: FC<PipelinePageActiveRunActionsProps> = ({ run }) => {
  const { toast } = useToast();
  const { mutate: signalRun, isPending: isSignaling } = useSignalRunMutation();

  const isResuming = getRunPauseSignal(run) === RunSignal.RESUME;
  const isBlocked = run.executionStatus?.observedState === ExecutionObservedState.BLOCKED;
  const isStopping = run.executionStatus?.desiredState === ExecutionDesiredState.STOPPED;

  const handleSignal = (signal: RunSignal) => {
    signalRun(
      create(SignalRunRequestSchema, {
        runId: run.id,
        signal,
        expectedRevision: run.executionStatus?.revision,
      }),
      {
        onError: (error) => {
          toast({
            header: "Run signal failed",
            description: getErrorMessage(error, "Failed to signal run"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  return (
    <>
      <PipelineRunStatus
        status={run.status}
        executionStatus={run.executionStatus}
        error={run.error}
      />
      <Button
        label={isResuming ? "Resume" : "Pause"}
        icon={isResuming ? PlayIcon : PauseIcon}
        iconWeight={IconWeight.FILL}
        variant={ButtonVariant.SECONDARY}
        isLoading={isSignaling}
        isDisabled={isStopping || isBlocked}
        onClick={() => handleSignal(getRunPauseSignal(run))}
      />
      <Button
        label="Stop"
        icon={StopIcon}
        iconWeight={IconWeight.FILL}
        variant={ButtonVariant.ERROR}
        isLoading={isSignaling}
        isDisabled={isStopping}
        onClick={() => handleSignal(getRunStopSignal(run))}
      />
    </>
  );
};

export default PipelinePageActiveRunActions;
