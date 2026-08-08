import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowUUpLeftIcon, FloppyDiskIcon, PlayIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, {
  type SelectInputOption,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import { ValidatePipelineRequestSchema } from "@/gen/ingestion/v1/capabilities_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Pipeline, PipelineSchedule, PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";
import { ListRunsRequestSchema, RunPipelineRequestSchema } from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/PipelineName";

import { hasPipelineGraphChanges, isPipelineRunnable } from "@/pages/pipelines/canvas/graph/diff";
import {
  mapCanvasStateToVersionRequest,
  mapPipelineVersionToCanvasState,
} from "@/pages/pipelines/canvas/graph/serialize";
import {
  usePipelineCanvasActions,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { usePipelineCanvasRunActions } from "@/pages/pipelines/canvas/providers/run/PipelineCanvasRunProvider";
import PipelineFlow from "@/pages/pipelines/components/flow/PipelineFlow";
import { mapCanvasNodesToFlowEndpoints } from "@/pages/pipelines/components/flow/utils";
import PipelineScheduleChip from "@/pages/pipelines/components/schedule/PipelineScheduleChip";
import { PIPELINE_NAVBAR_HEIGHT } from "@/pages/pipelines/layout/constants";
import { formatPipelineName, getPipelineValidationErrors } from "@/pages/pipelines/utils";

import { useValidatePipelineQuery } from "@/api/queries/capabilities";
import { ACTIVE_RUN_STATUSES } from "@/api/queries/constants";
import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { useRunPipelineMutation, useSuspenseListRunsQuery } from "@/api/queries/runs";

import { getErrorMessage } from "@/utils/errors";

const PipelineLayoutNavbarWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${PIPELINE_NAVBAR_HEIGHT}px;

  padding: 0 12px;

  display: flex;
  align-items: center;
  justify-content: space-between;

  flex-shrink: 0;

  background-color: ${({ theme }) => theme.color.background.base};
`);

interface PipelineLayoutNavbarProps {
  pipeline: Pipeline;
  schedule?: PipelineSchedule;
  currentVersion?: PipelineVersion;
  versions: PipelineVersion[];
  connections: Connection[];
  previewVersion: bigint | null;
  onPreviewVersionChange: (version: bigint | null) => void;
}

const PipelineLayoutNavbar = ({
  pipeline,
  schedule,
  currentVersion,
  versions,
  connections,
  previewVersion,
  onPreviewVersionChange,
}: PipelineLayoutNavbarProps) => {
  const { showToast } = useToast();

  const state = usePipelineCanvasState();
  const { loadGraph } = usePipelineCanvasActions();
  const { startRun } = usePipelineCanvasRunActions();
  const { mutate: createPipelineVersion, isPending: isSaving } = useCreatePipelineVersionMutation();
  const { mutate: runPipeline, isPending: isRunning } = useRunPipelineMutation();

  const { data: activeRunsData } = useSuspenseListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: pipeline.id,
      status: [...ACTIVE_RUN_STATUSES],
      limit: 1,
    }),
  });
  const hasActiveRun = activeRunsData.runs.length > 0;

  const validateInput = useMemo(
    () =>
      create(ValidatePipelineRequestSchema, {
        nodes: currentVersion?.nodes ?? [],
        edges: currentVersion?.edges ?? [],
      }),
    [currentVersion],
  );
  const { data: validation, isPending: isValidating } = useValidatePipelineQuery({
    input: validateInput,
  });
  const runErrors = useMemo(() => getPipelineValidationErrors(validation), [validation]);

  const hasChanges = useMemo(
    () => hasPipelineGraphChanges({ nodes: state.nodes, edges: state.edges }, currentVersion),
    [state.nodes, state.edges, currentVersion],
  );

  const handleUndo = () => {
    loadGraph(mapPipelineVersionToCanvasState(currentVersion, connections));
  };

  const handleSave = () => {
    createPipelineVersion(mapCanvasStateToVersionRequest(state, pipeline.id, currentVersion), {
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

  const { source, sinks } = useMemo(
    () => mapCanvasNodesToFlowEndpoints(state.nodes),
    [state.nodes],
  );
  const hasEdges = state.edges.length > 0;

  const isPreview = previewVersion !== null;
  const hasUnsavedChanges = !isPreview && hasChanges;

  const latestVersion = versions[0]?.version;
  const versionOptions = useMemo<SelectInputOption[]>(
    () =>
      versions.map((version) => ({
        id: version.version.toString(),
        label:
          version.version === latestVersion
            ? `Version ${version.version} - Latest`
            : `Version ${version.version}`,
        value: version.version,
      })),
    [versions, latestVersion],
  );
  const selectedVersionOption =
    versionOptions.find((option) => option.value === (previewVersion ?? latestVersion)) ?? null;

  const handleVersionChange = (option: SelectInputOption) => {
    const version = option.value as bigint;
    onPreviewVersionChange(version === latestVersion ? null : version);
  };

  const handleRun = () => {
    runPipeline(create(RunPipelineRequestSchema, { pipelineId: pipeline.id }), {
      onSuccess: (response) => {
        startRun(response.runs);
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

  return (
    <PipelineLayoutNavbarWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <PipelineFlow source={source} sinks={sinks} hasEdges={hasEdges} />
        <PipelineName pipelineId={pipeline.id} />
      </FlexWrapper>

      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        {versionOptions.length > 0 && (
          <SelectInput
            options={versionOptions}
            value={selectedVersionOption}
            onChange={handleVersionChange}
            size={InputSize.SMALL}
            variant={SelectInputVariant.SECONDARY}
            dropdownWidth={200}
            isDisabled={hasUnsavedChanges}
          />
        )}
        {isPreview && (
          <Button
            label="Back to latest"
            icon={ArrowUUpLeftIcon}
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={() => onPreviewVersionChange(null)}
          />
        )}
        {!isPreview && hasUnsavedChanges && (
          <Text size={TextSize.BODY_SM} variant={TextVariant.ERROR}>
            Unsaved changes
          </Text>
        )}
        {!isPreview &&
          (hasUnsavedChanges ? (
            <>
              <Button
                label="Undo"
                icon={ArrowUUpLeftIcon}
                variant={ButtonVariant.TERTIARY}
                size={ButtonSize.SMALL}
                onClick={handleUndo}
              />
              <Button
                label="Save"
                icon={FloppyDiskIcon}
                variant={ButtonVariant.SUCCESS}
                size={ButtonSize.SMALL}
                isLoading={isSaving}
                onClick={handleSave}
              />
            </>
          ) : (
            <>
              <PipelineScheduleChip schedule={schedule} />
              <Tooltip
                body={runErrors.join("\n")}
                position={TooltipPosition.BOTTOM}
                isDisabled={runErrors.length === 0}
              >
                <Button
                  label={hasActiveRun ? "Running..." : "Run"}
                  icon={PlayIcon}
                  variant={ButtonVariant.PRIMARY}
                  size={ButtonSize.SMALL}
                  isLoading={isRunning || isValidating}
                  isDisabled={
                    !isPipelineRunnable(currentVersion) || runErrors.length > 0 || hasActiveRun
                  }
                  onClick={handleRun}
                />
              </Tooltip>
            </>
          ))}
      </FlexWrapper>
    </PipelineLayoutNavbarWrapper>
  );
};

export default PipelineLayoutNavbar;
