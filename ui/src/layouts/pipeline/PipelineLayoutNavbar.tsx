import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import {
  ArrowUUpLeftIcon,
  FloppyDiskIcon,
  PauseIcon,
  PlayIcon,
  StopIcon,
  WarningIcon,
} from "@phosphor-icons/react";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Popover from "@galaxy-io/dls/overlays/Popover";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { Placement } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import { ExecutionMode, type WorkerConfiguration } from "@/gen/ingestion/v1/common_pb";
import {
  ExecutionDesiredState,
  ExecutionObservedState,
  type RunInfo,
  RunSignal,
  SignalRunRequestSchema,
} from "@/gen/ingestion/v1/runs_pb";

import { formatPipelineName } from "@/components/pipelines/utils";
import PipelineRunStatus from "@/components/runs/PipelineRunStatus";

import PipelineLayoutNavbarRunButton from "@/layouts/pipeline/PipelineLayoutNavbarRunButton";
import PipelineLayoutNavbarSaveIssues from "@/layouts/pipeline/PipelineLayoutNavbarSaveIssues";

import { isPipelineRunnable } from "@/pages/pipelines/canvas/graph/diff";
import { getPipelineGraphConflicts } from "@/pages/pipelines/canvas/graph/rules";
import {
  mapCanvasStateToVersionRequest,
  mapPipelineVersionToCanvasState,
} from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import {
  type PipelineCanvasValidationIssue,
  PipelineCanvasValidationIssueKind,
  usePipelineCanvasValidation,
} from "@/pages/pipelines/canvas/hooks/usePipelineCanvasValidation";
import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";
import {
  usePipelineCanvasActions,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { usePipelineCanvasNavigate } from "@/pages/pipelines/hooks/usePipelineCanvasNavigate";
import { usePipelineHasUnsavedChanges } from "@/pages/pipelines/hooks/usePipelineHasUnsavedChanges";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";
import { usePipelineRun } from "@/pages/pipelines/hooks/usePipelineRun";
import {
  getPipelineValidationErrors,
  getRunPauseSignal,
  getRunStopSignal,
} from "@/pages/pipelines/utils";

import { usePipelineParams } from "@/module/hooks";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import { useSignalRunMutation, useSuspenseListActivePipelineRunsQuery } from "@/api/queries/runs";

import { getErrorMessage } from "@/utils/errors";
import { isContinuousRunActive } from "@/utils/runs";

const PipelineLayoutNavbar: FC = () => {
  const { toast } = useToast();
  const navigateCanvas = usePipelineCanvasNavigate();
  const { id } = usePipelineParams();

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const isPreview = usePipelinePreviewVersion() !== undefined;
  const hasUnsavedChanges = usePipelineHasUnsavedChanges();

  const pipeline = pipelineData.pipeline;
  const isContinuous = pipeline?.executionMode === ExecutionMode.CONTINUOUS;
  const currentVersion = pipelineData.pipeline?.currentVersion;

  const state = usePipelineCanvasState();
  const { loadGraph } = usePipelineCanvasActions();
  const connectionByNodeId = usePipelineCanvasConnections();
  const showActivity = () =>
    navigateCanvas({
      node: undefined,
      resource: undefined,
      showPanel: true,
      tab: PipelineCanvasPanelTab.ACTIVITY,
    });
  const showResource = (edgeId: string) =>
    navigateCanvas({ node: undefined, resource: edgeId, showPanel: true, tab: undefined });
  const { mutate: createPipelineVersion, isPending: isSaving } = useCreatePipelineVersionMutation();
  const { startRun, isRunning } = usePipelineRun();
  const { mutate: signalRun, isPending: isSignaling } = useSignalRunMutation();

  const { data: activeRunsData } = useSuspenseListActivePipelineRunsQuery(id);
  const activeRun = activeRunsData.runs.find((run) => !isContinuous || isContinuousRunActive(run));
  const isResuming = activeRun && getRunPauseSignal(activeRun) === RunSignal.RESUME;
  const isBlocked = activeRun?.executionStatus?.observedState === ExecutionObservedState.BLOCKED;
  const isStopping = activeRun?.executionStatus?.desiredState === ExecutionDesiredState.STOPPED;

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
  } = useValidatePipelineQuery({
    input: validateInput,
  });
  const graphConflicts = useMemo(
    () => getPipelineGraphConflicts(state.edges, connectionByNodeId),
    [state.edges, connectionByNodeId],
  );
  const { issues, isPending: isValidatingCanvas, isError } = usePipelineCanvasValidation();
  const saveIssues = useMemo(
    () => [
      ...graphConflicts.map<PipelineCanvasValidationIssue>((message) => ({
        kind: PipelineCanvasValidationIssueKind.GRAPH,
        message,
      })),
      ...issues,
      ...(isError
        ? [
            {
              kind: PipelineCanvasValidationIssueKind.GRAPH,
              message: "Unable to validate this pipeline.",
            },
          ]
        : []),
    ],
    [graphConflicts, issues, isError],
  );

  const runErrors = useMemo(
    () => (validationError ? [validationError.message] : getPipelineValidationErrors(validation)),
    [validation, validationError],
  );

  if (!pipeline || isPreview) return null;

  const handleUndo = () => {
    loadGraph(mapPipelineVersionToCanvasState(currentVersion));
  };

  const handleSave = () => {
    createPipelineVersion(
      mapCanvasStateToVersionRequest(state, id, currentVersion, pipeline.executionMode),
      {
        onSuccess: () => {
          toast({
            header: "Pipeline saved",
            description: `${formatPipelineName(pipeline)} has been saved successfully.`,
            variant: ToastVariant.SUCCESS,
          });
        },
        onError: (error) => {
          toast({
            header: "Save failed",
            description: getErrorMessage(error, "Failed to save pipeline"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
  };

  const handleRun = (workerConfiguration?: WorkerConfiguration) => {
    startRun(pipeline, { workerConfiguration, onSuccess: showActivity });
  };

  const handleSignal = (runId: RunInfo["id"], signal: RunSignal) => {
    signalRun(
      create(SignalRunRequestSchema, {
        runId,
        signal,
        expectedRevision: activeRun?.executionStatus?.revision,
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
    <Flex alignItems={AlignItems.CENTER} gap={8}>
      {hasUnsavedChanges && (
        <Text size={TextSize.BODY_SM} variant={TextVariant.WARNING}>
          Unsaved changes
        </Text>
      )}
      {hasUnsavedChanges ? (
        <>
          <Button
            label="Undo"
            icon={ArrowUUpLeftIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={handleUndo}
          />
          {saveIssues.length > 0 && (
            <Popover
              placement={Placement.BOTTOM_END}
              ariaLabel="Save issues"
              body={
                <PipelineLayoutNavbarSaveIssues
                  issues={saveIssues}
                  onSelectResource={showResource}
                />
              }
            >
              <Button
                label={saveIssues.length.toString()}
                ariaLabel={pluralize("save issue", saveIssues.length, true)}
                leading={<Icon component={WarningIcon} variant={IconVariant.ERROR} />}
                variant={ButtonVariant.TERTIARY}
                size={ButtonSize.SMALL}
              />
            </Popover>
          )}
          <Button
            label="Save"
            icon={FloppyDiskIcon}
            variant={ButtonVariant.PRIMARY}
            size={ButtonSize.SMALL}
            isLoading={isSaving || isValidatingCanvas}
            isDisabled={saveIssues.length > 0}
            onClick={handleSave}
          />
        </>
      ) : (
        <>
          {!activeRun ? (
            <PipelineLayoutNavbarRunButton
              workerConfiguration={pipeline?.workerConfiguration}
              runErrors={runErrors}
              isRunnable={isPipelineRunnable(currentVersion)}
              isRunning={isRunning || isValidating}
              onRun={handleRun}
            />
          ) : (
            <>
              <PipelineRunStatus
                status={activeRun.status}
                executionStatus={activeRun.executionStatus}
                error={activeRun.error}
              />
              <Button
                label={isResuming ? "Resume" : "Pause"}
                icon={isResuming ? PlayIcon : PauseIcon}
                iconWeight={IconWeight.FILL}
                variant={ButtonVariant.SECONDARY}
                size={ButtonSize.SMALL}
                isLoading={isSignaling}
                isDisabled={isStopping || isBlocked}
                onClick={() => handleSignal(activeRun.id, getRunPauseSignal(activeRun))}
              />
              <Button
                label="Stop"
                icon={StopIcon}
                iconWeight={IconWeight.FILL}
                variant={ButtonVariant.ERROR}
                size={ButtonSize.SMALL}
                isLoading={isSignaling}
                isDisabled={isStopping}
                onClick={() => handleSignal(activeRun.id, getRunStopSignal(activeRun))}
              />
            </>
          )}
        </>
      )}
    </Flex>
  );
};

export default PipelineLayoutNavbar;
