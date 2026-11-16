import type { FC } from "react";

import { create } from "@bufbuild/protobuf";
import { PauseIcon, PlayIcon, StopIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
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
  isStream?: boolean;
  isCompact?: boolean;
}

const PipelinePageActiveRunActions: FC<PipelinePageActiveRunActionsProps> = ({
  run,
  isStream = false,
  isCompact = false,
}) => {
  const buttonSize = isCompact ? ButtonSize.SMALL : ButtonSize.MEDIUM;
  const { toast } = useToast();
  const { mutate: signalRun, isPending: isSignaling, variables } = useSignalRunMutation();

  const pauseSignal = getRunPauseSignal(run);
  const isResuming = pauseSignal === RunSignal.RESUME;
  const isBlocked = run.executionStatus?.observedState === ExecutionObservedState.BLOCKED;
  const isStopping = run.executionStatus?.desiredState === ExecutionDesiredState.STOPPED;
  const stopSignal = getRunStopSignal(run);
  const pendingSignal = isSignaling ? variables?.signal : undefined;

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
      {!isCompact && (
        <PipelineRunStatus
          status={run.status}
          executionStatus={run.executionStatus}
          error={run.error}
        />
      )}
      <Button
        label={`${isResuming ? "Resume" : "Pause"}${isStream ? " stream" : ""}`}
        icon={isResuming ? PlayIcon : PauseIcon}
        iconWeight={IconWeight.FILL}
        variant={ButtonVariant.SECONDARY}
        size={buttonSize}
        isLoading={pendingSignal === pauseSignal}
        isDisabled={isStopping || isBlocked || pendingSignal === stopSignal}
        onClick={() => handleSignal(pauseSignal)}
      />
      <Button
        label={isStream ? "Stop stream" : "Stop"}
        icon={StopIcon}
        iconWeight={IconWeight.FILL}
        variant={ButtonVariant.ERROR}
        size={buttonSize}
        isLoading={pendingSignal === stopSignal}
        isDisabled={isStopping || pendingSignal === pauseSignal}
        onClick={() => handleSignal(stopSignal)}
      />
    </>
  );
};

export default PipelinePageActiveRunActions;
