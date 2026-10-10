import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { ArrowLeftIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import SelectInput, { SelectInputSize, type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import EditableText from "@galaxy-io/dls/text/EditableText";
import { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import { ToastVariant } from "@galaxy-io/dls/toast/Toast";
import { useToast } from "@galaxy-io/dls/toast/useToast";

import {
  type UpdatePipelineRequest,
  UpdatePipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import { PIPELINE_UNTITLED_NAME } from "@/components/pipelines/constants";
import PipelineFlow from "@/components/pipelines/PipelineFlow";
import RouterLink from "@/components/RouterLink";

import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { mapCanvasNodesToFlowEndpoints } from "@/pages/pipelines/canvas/utils";
import { PIPELINE_PAGE_VERSION_SELECT_WIDTH } from "@/pages/pipelines/components/header/constants";
import { usePipelineCanvasNavigate } from "@/pages/pipelines/hooks/usePipelineCanvasNavigate";
import { usePipelineHasUnsavedChanges } from "@/pages/pipelines/hooks/usePipelineHasUnsavedChanges";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

import { usePipelineParams } from "@/module/hooks";
import { createFilamentHref, FilamentPath } from "@/module/paths";

import {
  createGetPipelineInput,
  useSuspenseGetPipelineQuery,
  useUpdatePipelineMutation,
} from "@/api/queries/pipelines";

import { getErrorMessage } from "@/utils/errors";
import { formatVersion } from "@/utils/format";
import { getNameError, isNameValid } from "@/utils/validation";

const PipelinePageHeading: FC = () => {
  const { id } = usePipelineParams();
  const state = usePipelineCanvasState();
  const navigateCanvas = usePipelineCanvasNavigate();
  const previewed = usePipelinePreviewVersion();
  const hasUnsavedChanges = usePipelineHasUnsavedChanges();
  const { toast } = useToast();
  const { mutate: updatePipeline } = useUpdatePipelineMutation();

  const { data: pipelineData } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });

  const pipeline = pipelineData.pipeline;
  const versions = pipeline?.versions ?? [];
  const latestVersion = versions[0]?.version;

  const { sourceId, sinkIds } = useMemo(
    () => mapCanvasNodesToFlowEndpoints(state.nodes),
    [state.nodes],
  );

  const versionOptions = useMemo<SelectOption[]>(
    () =>
      versions.map((version) => ({
        id: version.version.toString(),
        label: formatVersion(version.version),
      })),
    [versions],
  );

  if (!pipeline) {
    return null;
  }

  const handleUpdate = (patch: Pick<UpdatePipelineRequest, "name" | "description">) => {
    updatePipeline(
      create(UpdatePipelineRequestSchema, {
        pipelineId: pipeline.id,
        executionMode: pipeline.executionMode,
        ...patch,
      }),
      {
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

  const handleNameChange = (value: string) => {
    const name = value.trim();
    if (!isNameValid(name)) {
      toast({
        header: "Rename failed",
        description: getNameError(name, true),
        variant: ToastVariant.ERROR,
      });
      return;
    }
    handleUpdate({ name, description: pipeline.description });
  };

  const handleDescriptionChange = (value: string) => {
    handleUpdate({ name: pipeline.name, description: value.trim() });
  };

  const handleVersionChange = (versionId: string | null) => {
    const version = versions.find((item) => item.version.toString() === versionId)?.version;
    if (version === undefined) return;
    navigateCanvas({ version: version === latestVersion ? undefined : version });
  };

  return (
    <Flex alignItems={AlignItems.CENTER} gap={12} minWidth={0}>
      <Button
        icon={ArrowLeftIcon}
        variant={ButtonVariant.TERTIARY}
        ariaLabel="All pipelines"
        tooltip="All pipelines"
        href={createFilamentHref(FilamentPath.PIPELINES)}
        as={RouterLink}
      />
      <FlexItem shrink={0}>
        <PipelineFlow sourceId={sourceId} sinkIds={sinkIds} hasEdges={state.edges.length > 0} />
      </FlexItem>
      <Box as="h1" minWidth={0}>
        <EditableText
          ariaLabel="Pipeline name"
          value={pipeline.name}
          placeholder={PIPELINE_UNTITLED_NAME}
          onChange={handleNameChange}
          size={TextSize.HEADING_MD}
          weight={TextWeight.MEDIUM}
        />
      </Box>
      {versionOptions.length > 0 && (
        <FlexItem shrink={0} width={PIPELINE_PAGE_VERSION_SELECT_WIDTH}>
          <SelectInput
            ariaLabel="Version"
            options={versionOptions}
            value={(previewed?.version ?? latestVersion)?.toString() ?? null}
            onChange={handleVersionChange}
            size={SelectInputSize.SMALL}
            isDisabled={hasUnsavedChanges}
            fillWidth
          />
        </FlexItem>
      )}
      <FlexItem grow={1} minWidth={0}>
        <EditableText
          ariaLabel="Pipeline description"
          value={pipeline.description}
          placeholder="Add a description..."
          onChange={handleDescriptionChange}
        />
      </FlexItem>
    </Flex>
  );
};

export default PipelinePageHeading;
