import { type FC, useState } from "react";

import { PlayIcon, WarningIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import { IconWeight } from "@galaxy-io/dls/icons/Icon";
import CheckboxGroup, { CheckboxGroupSize } from "@galaxy-io/dls/inputs/CheckboxGroup";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import BulletedList, { BulletedListSize } from "@galaxy-io/dls/lists/BulletedList";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { WorkerConfiguration } from "@/gen/ingestion/v1/common_pb";

import BaseHeader, { BaseHeaderSize } from "@/components/BaseHeader";

import { PIPELINE_PAGE_RUN_DROPDOWN_WIDTH } from "@/pages/pipelines/components/header/constants";
import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";
import {
  formatWorkerConfiguration,
  parseWorkerConfiguration,
} from "@/pages/pipelines/components/worker/utils";

interface PipelinePageRunButtonProps {
  workerConfiguration?: WorkerConfiguration;
  resources: string[];
  runErrors: string[];
  isRunnable: boolean;
  isRunning: boolean;
  onRun: (workerConfiguration?: WorkerConfiguration, resyncResources?: string[]) => void;
}

interface PipelinePageRunButtonState {
  workerConfiguration: string;
  resyncResources: string[];
}

const PipelinePageRunButton: FC<PipelinePageRunButtonProps> = ({
  workerConfiguration,
  resources,
  runErrors,
  isRunnable,
  isRunning,
  onRun,
}) => {
  const [state, setState] = useState<PipelinePageRunButtonState>(() => ({
    workerConfiguration: formatWorkerConfiguration(workerConfiguration),
    resyncResources: [],
  }));

  const parsed = parseWorkerConfiguration(state.workerConfiguration);
  const resyncOptions = resources.map((resource) => ({ id: resource, label: resource }));

  return (
    <Button
      label="Run"
      icon={PlayIcon}
      iconWeight={IconWeight.FILL}
      variant={ButtonVariant.PRIMARY}
      isLoading={isRunning}
      isDisabled={!isRunnable || runErrors.length > 0}
      tooltip={
        runErrors.length > 0 ? <BulletedList items={runErrors} size={BulletedListSize.SMALL} /> : ""
      }
      onClick={() => onRun()}
      dropdown={({ close }) => (
        <Box variant={BoxVariant.PRIMARY} width={PIPELINE_PAGE_RUN_DROPDOWN_WIDTH}>
          <Flex alignItems={AlignItems.START} padding={12}>
            <BaseHeader title="Custom run configuration" size={BaseHeaderSize.SMALL} />
          </Flex>
          <Divider />
          <Flex
            alignItems={AlignItems.STRETCH}
            direction={FlexDirection.COLUMN}
            gap={12}
            padding={12}
          >
            <Widget isCollapsible header="Worker configuration" defaultIsOpen>
              <PipelineWorkerConfigurationEditor
                value={state.workerConfiguration}
                onChange={(value) => setState((prev) => ({ ...prev, workerConfiguration: value }))}
                isPerRun
              />
            </Widget>
            {resyncOptions.length > 0 && (
              <Widget isCollapsible header="Resync resources">
                <CheckboxGroup
                  options={resyncOptions}
                  value={state.resyncResources}
                  onChange={(value) => setState((prev) => ({ ...prev, resyncResources: value }))}
                  description="Each checked resource is read from the beginning again by this run."
                  size={CheckboxGroupSize.SMALL}
                />
              </Widget>
            )}
          </Flex>
          <Divider />
          <Flex
            alignItems={AlignItems.CENTER}
            justifyContent={JustifyContent.END}
            gap={8}
            fillWidth
            padding={[8, 12]}
          >
            {parsed.error && (
              <Chip
                label="Invalid"
                icon={WarningIcon}
                variant={ChipVariant.ERROR}
                tooltip={parsed.error}
              />
            )}
            <Button
              label="Run custom"
              icon={PlayIcon}
              iconWeight={IconWeight.FILL}
              variant={ButtonVariant.PRIMARY}
              size={ButtonSize.SMALL}
              isLoading={isRunning}
              isDisabled={!!parsed.error}
              onClick={() => {
                onRun(parsed.configuration, state.resyncResources);
                close();
              }}
            />
          </Flex>
        </Box>
      )}
    />
  );
};

export default PipelinePageRunButton;
