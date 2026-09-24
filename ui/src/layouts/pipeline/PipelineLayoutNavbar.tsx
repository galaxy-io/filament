import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import {
  ArrowUUpLeftIcon,
  FloppyDiskIcon,
  PauseIcon,
  PlayIcon,
  StopIcon,
  WarningIcon,
} from "@phosphor-icons/react";
import { useNavigate, useParams } from "@tanstack/react-router";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";
import Tooltip, { TooltipPosition, TooltipVariant } from "@galaxy-io/dls/tooltip/Tooltip";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import type { WorkerConfiguration } from "@/gen/ingestion/v1/common_pb";
import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import { GetPipelineRequestSchema, type PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";
import {
  ListRunsRequestSchema,
  type RunInfo,
  RunPipelineRequestSchema,
  RunSignal,
  RunStatus,
  SignalRunRequestSchema,
} from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/PipelineName";

import {
  PIPELINE_NAVBAR_HEIGHT,
  PIPELINE_VERSION_SELECT_DROPDOWN_WIDTH,
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
import { usePipelineCanvasValidation } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasValidation";
import { PipelineCanvasPanelTab } from "@/pages/pipelines/canvas/panel/types";
import {
  usePipelineCanvasActions,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import PipelineFlow from "@/pages/pipelines/components/flow/PipelineFlow";
import { mapCanvasNodesToFlowEndpoints } from "@/pages/pipelines/components/flow/utils";
import PipelineScheduleChip from "@/pages/pipelines/components/schedule/PipelineScheduleChip";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";
import { formatPipelineName, getPipelineValidationErrors } from "@/pages/pipelines/utils";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { useSuspenseListConnectionsQuery } from "@/api/queries/connections";
import { ACTIVE_RUN_STATUSES } from "@/api/queries/constants";
import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { useGetPipelineQuery, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import {
  getActiveRunsRefetchInterval,
  useRunPipelineMutation,
  useSignalRunMutation,
  useSuspenseListRunsQuery,
} from "@/api/queries/runs";

import { getErrorMessage } from "@/utils/errors";

const PipelineLayoutNavbarWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${PIPELINE_NAVBAR_HEIGHT}px;

  padding: 0 12px;

  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;

  flex-shrink: 0;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const PipelineLayoutNavbar = () => {
  const { showToast } = useToast();
  const navigate = useNavigate();
  const { id } = useParams({ from: "/_app/pipelines/$id" });

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id, includeVersions: true }),
  });
  const { data: connectionsData } = useSuspenseListConnectionsQuery();
  const previewed = usePipelinePreviewVersion();

  const pipeline = pipelineData.pipeline;
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
  const activeRun = activeRunsData.runs[0];

  const validateInput = useMemo(
    () =>
      create(ValidatePipelineRequestSchema, {
        graph: currentVersion?.graph,
      }),
    [currentVersion],
  );
  const { data: validation, isPending: isValidating } = useValidatePipelineQuery({
    input: validateInput,
  });
  const graphConflicts = useMemo(
    () => getPipelineGraphConflicts(state.edges, connectionByNodeId),
    [state.edges, connectionByNodeId],
  );
  const { issues, isPending: isValidatingCanvas, isError } = usePipelineCanvasValidation();
  const saveIssues = useMemo(
    () => [
      ...graphConflicts.map((message) => ({ message })),
      ...issues,
      ...(isError ? [{ message: "Unable to validate this pipeline." }] : []),
    ],
    [graphConflicts, issues, isError],
  );

  const runErrors = useMemo(() => getPipelineValidationErrors(validation), [validation]);

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
  const versionOptions = useMemo<SelectInputOption[]>(
    () =>
      versions.map((version) => ({
        id: version.version.toString(),
        label: `Version ${version.version.toString()}`,
        value: version.version,
      })),
    [versions],
  );

  if (!pipeline) return null;

  const isPreview = previewVersion !== null;
  const hasUnsavedChanges = !isPreview && hasChanges;

  const selectedVersionOption =
    versionOptions.find((option) => option.value === (previewVersion ?? latestVersion)) ?? null;

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

  const handleVersionChange = (option: SelectInputOption) => {
    const version = option.value as PipelineVersion["version"];
    handlePreviewVersionChange(version === latestVersion ? null : version);
  };

  const handleUndo = () => {
    loadGraph(mapPipelineVersionToCanvasState(currentVersion));
  };

  const handleSave = () => {
    createPipelineVersion(mapCanvasStateToVersionRequest(state, id, currentVersion), {
      onSuccess: () => {
        showToast({
          header: "Pipeline saved",
          subheader: `${formatPipelineName(pipeline)} has been saved successfully.`,
          variant: ToastVariant.SUCCESS,
        });
      },
      onError: (error) => {
        showToast({
          header: "Save failed",
          subheader: getErrorMessage(error, "Failed to save pipeline"),
          variant: ToastVariant.ERROR,
        });
      },
    });
  };

  const handleRun = (workerConfiguration?: WorkerConfiguration) => {
    runPipeline(create(RunPipelineRequestSchema, { pipelineId: id, workerConfiguration }), {
      onSuccess: () => {
        showActivity();
        showToast({
          header: "Run started",
          subheader: `${formatPipelineName(pipeline)} is now running.`,
          variant: ToastVariant.SUCCESS,
        });
      },
      onError: (error) => {
        showToast({
          header: "Run failed",
          subheader: getErrorMessage(error, "Failed to run pipeline"),
          variant: ToastVariant.ERROR,
        });
      },
    });
  };

  const handleSignal = (runId: RunInfo["id"], signal: RunSignal) => {
    signalRun(create(SignalRunRequestSchema, { runId, signal }), {
      onError: (error) => {
        showToast({
          header: "Run signal failed",
          subheader: getErrorMessage(error, "Failed to signal run"),
          variant: ToastVariant.ERROR,
        });
      },
    });
  };

  return (
    <PipelineLayoutNavbarWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM} grow={1} minWidth={0}>
        <FlexItem shrink={0}>
          <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} />
        </FlexItem>
        <FlexItem shrink={0}>
          <PipelineName pipelineId={id} />
        </FlexItem>
        {versionOptions.length > 0 && (
          <FlexItem shrink={0}>
            <SelectInput
              options={versionOptions}
              value={selectedVersionOption}
              onChange={handleVersionChange}
              size={InputSize.SMALL}
              dropdownWidth={PIPELINE_VERSION_SELECT_DROPDOWN_WIDTH}
              isDisabled={hasUnsavedChanges}
            />
          </FlexItem>
        )}
        {pipeline.description && (
          <FlexItem grow={1} minWidth={0}>
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} isEllipsis>
              {pipeline.description}
            </Text>
          </FlexItem>
        )}
      </FlexWrapper>

      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM} shrink={0}>
        {isPreview && (
          <Button
            label="Back to latest"
            icon={ArrowUUpLeftIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={() => handlePreviewVersionChange(null)}
          />
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
                <Tooltip
                  variant={TooltipVariant.PRIMARY}
                  position={TooltipPosition.BOTTOM_END}
                  body={
                    <PipelineLayoutNavbarSaveIssues
                      issues={saveIssues}
                      onSelectResource={showResource}
                    />
                  }
                  isInteractive
                >
                  <Chip
                    label={saveIssues.length.toString()}
                    icon={WarningIcon}
                    variant={ChipVariant.ERROR}
                  />
                </Tooltip>
              )}
              <Button
                label="Save"
                icon={FloppyDiskIcon}
                variant={ButtonVariant.PRIMARY_ALT}
                size={ButtonSize.SMALL}
                isLoading={isSaving || isValidatingCanvas}
                isDisabled={saveIssues.length > 0}
                onClick={handleSave}
              />
            </>
          ) : (
            <>
              {!activeRun && <PipelineScheduleChip pipelineId={id} />}
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
                  <PipelineHistoryRunStatus status={activeRun.status} />
                  <Button
                    label={activeRun?.status === RunStatus.PAUSED ? "Resume" : "Pause"}
                    icon={activeRun?.status === RunStatus.PAUSED ? PlayIcon : PauseIcon}
                    variant={ButtonVariant.SECONDARY}
                    size={ButtonSize.SMALL}
                    isLoading={isSignaling}
                    onClick={() =>
                      handleSignal(
                        activeRun.id,
                        activeRun?.status === RunStatus.PAUSED ? RunSignal.RESUME : RunSignal.PAUSE,
                      )
                    }
                    isIconFilled
                  />
                  <Button
                    label="Stop"
                    icon={StopIcon}
                    variant={ButtonVariant.ERROR}
                    size={ButtonSize.SMALL}
                    isLoading={isSignaling}
                    onClick={() => handleSignal(activeRun.id, RunSignal.CANCEL)}
                    isIconFilled
                  />
                </>
              )}
            </>
          ))}
      </FlexWrapper>
    </PipelineLayoutNavbarWrapper>
  );
};

export default PipelineLayoutNavbar;
