import { type FC, useMemo } from "react";

import { ArrowLeftIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import SelectInput, { SelectInputSize, type SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import PipelineFlow from "@/components/pipelines/PipelineFlow";
import { formatPipelineName } from "@/components/pipelines/utils";
import RouterLink from "@/components/RouterLink";

import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { mapCanvasNodesToFlowEndpoints } from "@/pages/pipelines/canvas/utils";
import { PIPELINE_PAGE_VERSION_SELECT_WIDTH } from "@/pages/pipelines/components/header/constants";
import { usePipelineCanvasNavigate } from "@/pages/pipelines/hooks/usePipelineCanvasNavigate";
import { usePipelineHasUnsavedChanges } from "@/pages/pipelines/hooks/usePipelineHasUnsavedChanges";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

import { usePipelineParams } from "@/module/hooks";
import { createFilamentHref, FilamentPath } from "@/module/paths";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

import { formatVersion } from "@/utils/format";

const PipelinePageHeading: FC = () => {
  const { id } = usePipelineParams();
  const state = usePipelineCanvasState();
  const navigateCanvas = usePipelineCanvasNavigate();
  const previewed = usePipelinePreviewVersion();
  const hasUnsavedChanges = usePipelineHasUnsavedChanges();

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
      <Text as="h1" size={TextSize.HEADING_MD} weight={TextWeight.MEDIUM} lineClamp={1}>
        {formatPipelineName(pipeline)}
      </Text>
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
      {pipeline.description && (
        <FlexItem grow={1} minWidth={0}>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
            {pipeline.description}
          </Text>
        </FlexItem>
      )}
    </Flex>
  );
};

export default PipelinePageHeading;
