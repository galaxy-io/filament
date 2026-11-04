import { type FC, useCallback, useState } from "react";

import { CaretRightIcon, PlayIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import ErrorLayout from "@galaxy-io/dls/layout/ErrorLayout";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import PendingLayout from "@galaxy-io/dls/layout/PendingLayout";
import Modal, { ModalSize } from "@galaxy-io/dls/modal/Modal";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

import PipelineFlow, { PipelineFlowSize } from "@/components/pipelines/PipelineFlow";
import { formatPipelineName, mapVersionNodesToFlowEndpoints } from "@/components/pipelines/utils";

import {
  PIPELINE_RUN_MODAL_RESOURCES_CARET_SIZE,
  PIPELINE_RUN_MODAL_TITLE_ICON_SIZE,
} from "@/pages/pipelines/components/run/constants";
import PipelineRunModalResourcesPane from "@/pages/pipelines/components/run/PipelineRunModalResourcesPane";
import PipelineRunModalSectionChip from "@/pages/pipelines/components/run/PipelineRunModalSectionChip";
import { canPipelineRunSubset } from "@/pages/pipelines/components/run/utils";
import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";
import {
  formatWorkerConfiguration,
  isWorkerConfigurationEqual,
  parseWorkerConfiguration,
} from "@/pages/pipelines/components/worker/utils";
import { usePipelineRun } from "@/pages/pipelines/hooks/usePipelineRun";
import { getPipelineResources } from "@/pages/pipelines/utils";

import { useFilamentNavigate } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";
import type { FilamentLayoutSearch } from "@/module/schemas";

import { createGetConnectionInput, useGetConnectionQuery } from "@/api/queries/connections";
import { createGetPipelineInput, useGetPipelineQuery } from "@/api/queries/pipelines";

import { useOverlayRecord } from "@/hooks/useOverlayRecord";
import { useOverlaySession } from "@/hooks/useOverlaySession";

import { IS_DEBUG } from "@/constants";

import { isNotFoundError } from "@/utils/errors";

interface PipelineRunModalProps {
  isOpen: boolean;
  pipelineId?: Pipeline["id"];
  onClose: () => void;
}

interface PipelineRunModalContentProps {
  isOpen: boolean;
  pipeline: Pipeline;
  onClose: () => void;
}

interface PipelineRunModalState {
  workerConfiguration: string;
  selectedResources: string[];
  resyncResources: string[];
  isResourcesPaneOpen: boolean;
}

const PipelineRunModalContent: FC<PipelineRunModalContentProps> = ({
  isOpen,
  pipeline,
  onClose,
}) => {
  const navigate = useFilamentNavigate();
  const { startRun, isRunning } = usePipelineRun();
  const { workerConfiguration, currentVersion } = pipeline;
  const resources = getPipelineResources(currentVersion);
  const { sourceId, sinkIds } = mapVersionNodesToFlowEndpoints(currentVersion?.graph?.nodes ?? []);
  const pipelineName = formatPipelineName(pipeline);
  const { data: sourceData } = useGetConnectionQuery({
    input: createGetConnectionInput(sourceId ?? ""),
    options: { enabled: !!sourceId },
  });
  const canSubset = canPipelineRunSubset(pipeline, sourceData?.connection);
  const defaultWorkerConfiguration = formatWorkerConfiguration(workerConfiguration);
  const [state, setState] = useState<PipelineRunModalState>(() => ({
    workerConfiguration: defaultWorkerConfiguration,
    selectedResources: resources,
    resyncResources: [],
    isResourcesPaneOpen: false,
  }));

  const parsed = parseWorkerConfiguration(state.workerConfiguration);
  const isWorkerDefault =
    state.workerConfiguration === defaultWorkerConfiguration ||
    (!parsed.error && isWorkerConfigurationEqual(parsed.configuration, workerConfiguration));
  const runResources = canSubset ? state.selectedResources : resources;
  const isSubset = runResources.length < resources.length;
  const resyncResources = state.resyncResources.filter((resource) =>
    runResources.includes(resource),
  );
  const isResourcesDefault = !isSubset && resyncResources.length === 0;
  const isDefault = isWorkerDefault && isResourcesDefault;

  const handleSelectedResourcesChange = useCallback(
    (selectedResources: string[]) => setState((prev) => ({ ...prev, selectedResources })),
    [],
  );

  const handleResyncResourcesChange = useCallback(
    (resyncResources: string[]) => setState((prev) => ({ ...prev, resyncResources })),
    [],
  );

  const handleResourcesPaneToggle = (isResourcesPaneOpen: boolean) =>
    setState((prev) => ({ ...prev, isResourcesPaneOpen }));

  const handleWorkerConfigurationChange = (workerConfiguration: string) =>
    setState((prev) => ({ ...prev, workerConfiguration }));

  const handleReset = () =>
    setState((prev) => ({
      ...prev,
      workerConfiguration: defaultWorkerConfiguration,
      selectedResources: resources,
      resyncResources: [],
    }));

  const handleRun = () => {
    startRun(pipeline, {
      workerConfiguration: isWorkerDefault ? undefined : parsed.configuration,
      resources: isSubset ? runResources : [],
      resyncResources,
      onSuccess: () =>
        void navigate({
          to: FilamentPath.PIPELINE_CANVAS,
          params: { id: pipeline.id },
          search: ({ flow: _, pipelineId: __, ...rest }: FilamentLayoutSearch) => rest,
        }),
    });
  };

  return (
    <Modal
      isOpen={isOpen}
      header={
        <Flex alignItems={AlignItems.CENTER} gap={8}>
          <Icon
            component={PlayIcon}
            size={PIPELINE_RUN_MODAL_TITLE_ICON_SIZE}
            weight={IconWeight.FILL}
            variant={IconVariant.PRIMARY}
          />
          <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
            Run
          </Text>
          <PipelineFlow sourceId={sourceId} sinkIds={sinkIds} size={PipelineFlowSize.SMALL} />
          <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
            {pipelineName}
          </Text>
        </Flex>
      }
      ariaLabel={`Run ${pipelineName}`}
      size={ModalSize.MEDIUM}
      aside={
        state.isResourcesPaneOpen ? (
          <PipelineRunModalResourcesPane
            resources={resources}
            selectedResources={runResources}
            resyncResources={state.resyncResources}
            isDefault={isResourcesDefault}
            canSubset={canSubset}
            onSelectedChange={handleSelectedResourcesChange}
            onResyncChange={handleResyncResourcesChange}
            onClose={() => handleResourcesPaneToggle(false)}
          />
        ) : null
      }
      footer={
        <Flex justifyContent={JustifyContent.END} gap={8} fillWidth>
          <Button
            label="Reset"
            variant={ButtonVariant.SECONDARY}
            isDisabled={isDefault}
            onClick={handleReset}
          />
          <Button
            label={isDefault ? "Run now" : "Start custom run"}
            icon={PlayIcon}
            iconWeight={IconWeight.FILL}
            variant={ButtonVariant.PRIMARY}
            isLoading={isRunning}
            isDisabled={!!parsed.error || runResources.length === 0}
            onClick={handleRun}
          />
        </Flex>
      }
      hasDividers
      onOpenChange={(isOpen) => {
        if (!isOpen) onClose();
      }}
    >
      <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} gap={12}>
        <Widget
          isInteractive
          ariaLabel={state.isResourcesPaneOpen ? "Close resources" : "Open resources"}
          isDisabled={resources.length === 0}
          onClick={() => handleResourcesPaneToggle(!state.isResourcesPaneOpen)}
        >
          <Flex
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.SPACE_BETWEEN}
            gap={8}
          >
            <Text
              size={TextSize.BODY_MD}
              weight={TextWeight.MEDIUM}
              variant={state.isResourcesPaneOpen ? TextVariant.PRIMARY : TextVariant.SECONDARY}
            >
              Resources
            </Text>
            <Flex alignItems={AlignItems.CENTER} gap={8}>
              <PipelineRunModalSectionChip isDefault={isResourcesDefault} />
              <Icon
                component={CaretRightIcon}
                size={PIPELINE_RUN_MODAL_RESOURCES_CARET_SIZE}
                variant={IconVariant.SECONDARY}
              />
            </Flex>
          </Flex>
        </Widget>
        <Widget
          isCollapsible
          defaultIsOpen
          header="Worker configuration"
          actions={<PipelineRunModalSectionChip isDefault={isWorkerDefault} />}
        >
          <PipelineWorkerConfigurationEditor
            value={state.workerConfiguration}
            onChange={handleWorkerConfigurationChange}
            error={parsed.error}
            isPerRun
          />
        </Widget>
      </Flex>
    </Modal>
  );
};

const PipelineRunModal: FC<PipelineRunModalProps> = ({ isOpen, pipelineId, onClose }) => {
  const session = useOverlaySession(isOpen);
  const heldPipelineId = useOverlayRecord(isOpen, pipelineId);
  const { data, error, isError } = useGetPipelineQuery({
    input: createGetPipelineInput(heldPipelineId ?? ""),
    options: { enabled: !!session && !!heldPipelineId, retry: false },
  });

  if (!session) {
    return null;
  }

  if (isError) {
    const isNotFound = isNotFoundError(error);

    return (
      <Modal
        isOpen={isOpen}
        header="Run pipeline"
        size={ModalSize.MEDIUM}
        onOpenChange={(isOpen) => {
          if (!isOpen) onClose();
        }}
      >
        <ErrorLayout
          header={isNotFound ? "Pipeline not found" : "Failed to load pipeline"}
          description={
            isNotFound
              ? "This pipeline no longer exists."
              : "The pipeline could not be loaded. Try again later."
          }
          detail={IS_DEBUG ? error?.message : undefined}
          actions={<Button label="Close" onClick={onClose} variant={ButtonVariant.SECONDARY} />}
        />
      </Modal>
    );
  }

  if (!data?.pipeline) {
    return (
      <Modal
        isOpen={isOpen}
        header="Run pipeline"
        size={ModalSize.MEDIUM}
        onOpenChange={(isOpen) => {
          if (!isOpen) onClose();
        }}
      >
        <PendingLayout />
      </Modal>
    );
  }

  return (
    <PipelineRunModalContent
      key={session}
      isOpen={isOpen}
      pipeline={data.pipeline}
      onClose={onClose}
    />
  );
};

export default PipelineRunModal;
