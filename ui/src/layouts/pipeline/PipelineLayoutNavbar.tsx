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
import { useNavigate, useParams } from "@tanstack/react-router";
import pluralize from "pluralize";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import SelectInput, { SelectInputSize, type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Popover from "@galaxy-io/dls/overlays/Popover";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { Placement } from "@galaxy-io/dls/theme/enums";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import { ExecutionMode, type WorkerConfiguration } from "@/gen/ingestion/v1/common_pb";
import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import { GetPipelineRequestSchema, type PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";
import {
  ExecutionDesiredState,
  ExecutionObservedState,
  ListRunsRequestSchema,
  type RunInfo,
  RunPipelineRequestSchema,
  RunSignal,
  SignalRunRequestSchema,
} from "@/gen/ingestion/v1/runs_pb";

import PipelineFlow from "@/components/pipelines/PipelineFlow";
import PipelineName from "@/components/pipelines/PipelineName";
import PipelineScheduleChip from "@/components/pipelines/PipelineScheduleChip";
import { formatPipelineName } from "@/components/pipelines/utils";
import PipelineRunStatus from "@/components/runs/PipelineRunStatus";

import {
  PIPELINE_NAVBAR_HEIGHT,
  PIPELINE_NAVBAR_VERSION_SELECT_WIDTH,
} from "@/layouts/pipeline/constants";
import PipelineLayoutNavbarRunButton from "@/layouts/pipeline/PipelineLayoutNavbarRunButton";
import PipelineLayoutNavbarSaveIssues from "@/layouts/pipeline/PipelineLayoutNavbarSaveIssues";

import { hasPipelineGraphChanges, isPipelineRunnable } from "@/pages/pipelines/canvas/graph/diff";
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
import { mapCanvasNodesToFlowEndpoints } from "@/pages/pipelines/canvas/utils";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";
import {
  getPipelineValidationErrors,
  getRunPauseSignal,
  getRunStopSignal,
} from "@/pages/pipelines/utils";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { useGetPipelineQuery, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import {
  getActiveRunsRefetchInterval,
  useRunPipelineMutation,
  useSignalRunMutation,
  useSuspenseListRunsQuery,
} from "@/api/queries/runs";

import { ACTIVE_RUN_STATUSES } from "@/constants";

import { getErrorMessage } from "@/utils/errors";
import { formatVersion } from "@/utils/format";
import { isContinuousRunActive } from "@/utils/runs";

const PipelineLayoutNavbar: FC = () => {
  const { toast } = useToast();
  const navigate = useNavigate();
  const { id } = useParams({ from: "/_app/pipelines/$id" });

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id, includeVersions: true }),
  });
  const { data: connectionsData } = useSuspenseListConnectionsQuery();
  const previewed = usePipelinePreviewVersion();

  const pipeline = pipelineData.pipeline;
  const isContinuous = pipeline?.executionMode === ExecutionMode.CONTINUOUS;
  const currentVersion = pipelineData.pipeline?.currentVersion;
  const versions = pipelineData.pipeline?.versions ?? [];
  const previewVersion = previewed?.version ?? null;

  const state = usePipelineCanvasState();
  const { loadGraph } = usePipelineCanvasActions();
  const connectionByNodeId = usePipelineCanvasConnections();
  const showActivity = () =>
    void navigate({
      to: "/pipelines/$id/canvas",
      params: { id },
      search: (prev) => ({
        ...prev,
        node: undefined,
        resource: undefined,
        showPanel: true,
        tab: PipelineCanvasPanelTab.ACTIVITY,
      }),
    });
  const showResource = (edgeId: string) =>
    void navigate({
      to: "/pipelines/$id/canvas",
      params: { id },
      search: (prev) => ({
        ...prev,
        node: undefined,
        resource: edgeId,
        showPanel: true,
        tab: undefined,
      }),
    });
  const { mutate: createPipelineVersion, isPending: isSaving } = useCreatePipelineVersionMutation();
  const { mutate: runPipeline, isPending: isRunning } = useRunPipelineMutation();
  const { mutate: signalRun, isPending: isSignaling } = useSignalRunMutation();

  const { data: schedulePipelineData } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id, includeSchedule: true }),
  });
  const schedule = schedulePipelineData?.pipeline?.schedule;
  const nextFireAt = schedule?.config?.isEnabled ? schedule.nextFireAt : undefined;

  const { data: activeRunsData } = useSuspenseListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: id,
      status: [...ACTIVE_RUN_STATUSES],
      pagination: create(PaginationRequestSchema, { pageSize: 1 }),
    }),
    options: {
      refetchInterval: (query) => getActiveRunsRefetchInterval(query.state.data?.runs, nextFireAt),
    },
  });
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

  const hasChanges = useMemo(
    () => hasPipelineGraphChanges({ nodes: state.nodes, edges: state.edges }, currentVersion),
    [state.nodes, state.edges, currentVersion],
  );

  const { source, sinks } = useMemo(
    () => mapCanvasNodesToFlowEndpoints(state.nodes, connectionsData.connections),
    [state.nodes, connectionsData.connections],
  );
  const hasEdges = state.edges.length > 0;

  const latestVersion = versions[0]?.version;
  const versionOptions = useMemo<SelectOption[]>(
    () =>
      versions.map((version) => ({
        id: version.version.toString(),
        label: formatVersion(version.version),
      })),
    [versions],
  );

  if (!pipeline) return null;

  const isPreview = previewVersion !== null;
  const hasUnsavedChanges = !isPreview && hasChanges;

  const selectedVersionId = (previewVersion ?? latestVersion)?.toString() ?? null;

  const handlePreviewVersionChange = (nextVersion: PipelineVersion["version"] | null) => {
    void navigate({
      to: "/pipelines/$id/canvas",
      params: { id },
      search: (prev) => ({
        ...prev,
        version: nextVersion ?? undefined,
      }),
    });
  };

  const handleVersionChange = (versionId: string | null) => {
    const version = versions.find((item) => item.version.toString() === versionId)?.version;
    if (version === undefined) return;
    handlePreviewVersionChange(version === latestVersion ? null : version);
  };

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
    runPipeline(
      create(RunPipelineRequestSchema, {
        pipelineId: id,
        workerConfiguration,
        options: { executionMode: pipeline.executionMode },
      }),
      {
        onSuccess: () => {
          showActivity();
          toast({
            header: "Run started",
            description: `${formatPipelineName(pipeline)} is now running.`,
            variant: ToastVariant.SUCCESS,
          });
        },
        onError: (error) => {
          toast({
            header: "Run failed",
            description: getErrorMessage(error, "Failed to run pipeline"),
            variant: ToastVariant.ERROR,
          });
        },
      },
    );
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
    <Flex
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={12}
      fillWidth
      height={PIPELINE_NAVBAR_HEIGHT}
      shrink={0}
      padding={[0, 12]}
    >
      <Flex alignItems={AlignItems.CENTER} gap={12} grow={1} minWidth={0}>
        <FlexItem shrink={0}>
          <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} />
        </FlexItem>
        <FlexItem shrink={0}>
          <PipelineName pipelineId={id} />
        </FlexItem>
        {versionOptions.length > 0 && (
          <FlexItem shrink={0} width={PIPELINE_NAVBAR_VERSION_SELECT_WIDTH}>
            <SelectInput
              options={versionOptions}
              value={selectedVersionId}
              onChange={handleVersionChange}
              size={SelectInputSize.SMALL}
              isDisabled={hasUnsavedChanges}
              fillWidth
            />
          </FlexItem>
        )}
        {pipeline.description && (
          <FlexItem grow={1} minWidth={0}>
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
              {pipeline.description}
            </Text>
          </FlexItem>
        )}
      </Flex>

      <Flex alignItems={AlignItems.CENTER} gap={12} shrink={0}>
        {isPreview && (
          <>
            <Button
              label="Back to latest"
              icon={ArrowUUpLeftIcon}
              variant={ButtonVariant.TERTIARY}
              size={ButtonSize.SMALL}
              onClick={() => handlePreviewVersionChange(null)}
            />
            <Chip label={formatVersion(previewVersion)} variant={ChipVariant.ERROR} />
          </>
        )}
        {!isPreview && hasUnsavedChanges && (
          <Text size={TextSize.BODY_SM} variant={TextVariant.WARNING}>
            Unsaved changes
          </Text>
        )}
        {!isPreview &&
          (hasUnsavedChanges ? (
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
              {!activeRun && !isContinuous && <PipelineScheduleChip pipelineId={id} />}
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
          ))}
      </Flex>
    </Flex>
  );
};

export default PipelineLayoutNavbar;
