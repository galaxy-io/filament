import { useState } from "react";

import { PlayIcon, WarningIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import { IconWeight } from "@galaxy-io/dls/icons/Icon";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { Placement } from "@galaxy-io/dls/theme/enums";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { WorkerConfiguration } from "@/gen/ingestion/v1/common_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";
import { PIPELINE_NAVBAR_RUN_DROPDOWN_WIDTH } from "@/layouts/pipeline/constants";

import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";
import {
  formatWorkerConfiguration,
  parseWorkerConfiguration,
} from "@/pages/pipelines/components/worker/utils";

interface PipelineLayoutNavbarRunButtonProps {
  workerConfiguration?: WorkerConfiguration;
  runErrors: string[];
  isRunnable: boolean;
  isRunning: boolean;
  onRun: (workerConfiguration?: WorkerConfiguration) => void;
}

export interface PipelineLayoutNavbarRunButtonState {
  workerConfiguration: string;
}

const PipelineLayoutNavbarRunButton = ({
  workerConfiguration,
  runErrors,
  isRunnable,
  isRunning,
  onRun,
}: PipelineLayoutNavbarRunButtonProps) => {
  const [state, setState] = useState<PipelineLayoutNavbarRunButtonState>(() => ({
    workerConfiguration: formatWorkerConfiguration(workerConfiguration),
  }));

  const parsed = parseWorkerConfiguration(state.workerConfiguration);

  return (
    <Tooltip
      body={runErrors.join("\n")}
      placement={Placement.BOTTOM}
      isDisabled={runErrors.length === 0}
    >
      <Button
        label="Run"
        icon={PlayIcon}
        iconWeight={IconWeight.FILL}
        variant={ButtonVariant.PRIMARY}
        size={ButtonSize.SMALL}
        isLoading={isRunning}
        isDisabled={!isRunnable || runErrors.length > 0}
        onClick={() => onRun()}
        dropdown={({ close }) => (
          <Box variant={BoxVariant.PRIMARY} width={PIPELINE_NAVBAR_RUN_DROPDOWN_WIDTH}>
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
                  onChange={(value) =>
                    setState((prev) => ({ ...prev, workerConfiguration: value }))
                  }
                  help="Applied to the Kubernetes Job for this run only"
                />
              </Widget>
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
                  onRun(parsed.configuration);
                  close();
                }}
              />
            </Flex>
          </Box>
        )}
      />
    </Tooltip>
  );
};

export default PipelineLayoutNavbarRunButton;
