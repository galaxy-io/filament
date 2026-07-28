import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowUUpLeftIcon, FloppyDiskIcon, PlayIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import SelectInput, {
  type SelectInputOption,
  SelectInputSize,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import type { Pipeline, PipelineVersion } from "@/gen/ingestion/v1/pipelines_pb";
import { RunPipelineRequestSchema } from "@/gen/ingestion/v1/runs_pb";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/actions";
import {
  hasPipelineGraphChanges,
  isPipelineRunnable,
  mapCanvasStateToVersionRequest,
} from "@/pages/pipelines/canvas/graph";
import { usePipelineCanvas } from "@/pages/pipelines/canvas/PipelineCanvasProvider";
import PipelineFlow from "@/pages/pipelines/components/flow/PipelineFlow";
import { mapCanvasNodesToFlowEndpoints } from "@/pages/pipelines/components/flow/utils";
import { PIPELINE_NAVBAR_HEIGHT } from "@/pages/pipelines/layout/constants";
import { formatPipelineName } from "@/pages/pipelines/utils";

import { useCreatePipelineVersionMutation } from "@/api/queries/pipeline_versions";
import { useRunPipelineMutation } from "@/api/queries/runs";

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
  currentVersion?: PipelineVersion;
  versions: PipelineVersion[];
  previewVersion: bigint | null;
  onPreviewVersionChange: (version: bigint | null) => void;
}

const PipelineLayoutNavbar = ({
  pipeline,
  currentVersion,
  versions,
  previewVersion,
  onPreviewVersionChange,
}: PipelineLayoutNavbarProps) => {
  const { showToast } = useToast();

  const { state: canvasState, dispatch } = usePipelineCanvas();
  const { mutate: createPipelineVersion, isPending: isSaving } = useCreatePipelineVersionMutation();
  const { mutate: runPipeline, isPending: isRunning } = useRunPipelineMutation();

  const hasChanges = useMemo(
    () => hasPipelineGraphChanges(canvasState, currentVersion),
    [canvasState, currentVersion],
  );

  const handleSave = () => {
    createPipelineVersion(
      mapCanvasStateToVersionRequest(canvasState, pipeline.id, currentVersion),
      {
        onSuccess: () => {
          showToast({
            header: "Pipeline saved",
            subheader: `${pipeline.name} has been saved successfully.`,
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
      },
    );
  };

  const { source, sinks } = useMemo(
    () => mapCanvasNodesToFlowEndpoints(canvasState.nodes),
    [canvasState.nodes],
  );
  const hasEdges = canvasState.edges.length > 0;

  const isPreview = previewVersion !== null;
  const hasUnsavedChanges = !isPreview && hasChanges;

  const latestVersion = versions[0]?.version;
  const versionOptions = useMemo<SelectInputOption[]>(
    () =>
      versions.map((version) => ({
        id: version.version.toString(),
        label:
          version.version === latestVersion
            ? `Version ${version.version} • Latest`
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
        dispatch({
          type: PipelineCanvasActionType.SET_RUN_BINDINGS,
          payload: response.runs,
        });
        dispatch({
          type: PipelineCanvasActionType.SET_ACTIVITY_OPEN,
          payload: true,
        });
        showToast({
          header: "Run started",
          subheader: `${pipeline.name} is now running.`,
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
        <Text size={TextSize.BODY_LG}>{formatPipelineName(pipeline.name)}</Text>
      </FlexWrapper>

      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        {versionOptions.length > 0 && (
          <SelectInput
            options={versionOptions}
            value={selectedVersionOption}
            onChange={handleVersionChange}
            size={SelectInputSize.SMALL}
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
            <Button
              label="Save"
              icon={FloppyDiskIcon}
              variant={ButtonVariant.SECONDARY}
              size={ButtonSize.SMALL}
              isLoading={isSaving}
              onClick={handleSave}
            />
          ) : (
            <Button
              label="Run"
              icon={PlayIcon}
              variant={ButtonVariant.PRIMARY}
              size={ButtonSize.SMALL}
              isLoading={isRunning}
              isDisabled={!isPipelineRunnable(currentVersion)}
              onClick={handleRun}
            />
          ))}
      </FlexWrapper>
    </PipelineLayoutNavbarWrapper>
  );
};

export default PipelineLayoutNavbar;
