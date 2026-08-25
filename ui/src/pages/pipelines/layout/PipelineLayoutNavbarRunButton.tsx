import { useState } from "react";

import { styled } from "@linaria/react";
import { PlayIcon, WarningIcon } from "@phosphor-icons/react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { type PropsWithTheme, withTheme } from "@galaxy-io/dls/theme";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import type { WorkerConfiguration } from "@/gen/ingestion/v1/common_pb";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

import PipelineWorkerConfigurationEditor from "@/pages/pipelines/components/worker/PipelineWorkerConfigurationEditor";
import {
  formatWorkerConfiguration,
  parseWorkerConfiguration,
} from "@/pages/pipelines/components/worker/utils";
import { PIPELINE_NAVBAR_RUN_DROPDOWN_WIDTH } from "@/pages/pipelines/layout/constants";

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

const PipelineLayoutNavbarRunButtonDropdown = withTheme(styled.div<PropsWithTheme>`
  background-color: ${({ theme }) => theme.color.background.primary};
  width: ${PIPELINE_NAVBAR_RUN_DROPDOWN_WIDTH}px;
  display: flex;
  flex-direction: column;
`);

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
      position={TooltipPosition.BOTTOM}
      isDisabled={runErrors.length === 0}
    >
      <Button
        label="Run"
        icon={PlayIcon}
        variant={ButtonVariant.PRIMARY}
        size={ButtonSize.SMALL}
        isLoading={isRunning}
        isDisabled={!isRunnable || runErrors.length > 0}
        onClick={() => onRun()}
        contentWhenDropdown={({ close }) => (
          <PipelineLayoutNavbarRunButtonDropdown>
            <FlexWrapper padding="12px">
              <BaseHeader title="Custom run configuration" size={BaseHeaderSize.SMALL} />
            </FlexWrapper>
            <HorizontalDivider />
            <FlexWrapper direction={FlexDirection.COLUMN} gap={12} padding="12px">
              <Accordion header="Worker configuration" isOpenInitial>
                <PipelineWorkerConfigurationEditor
                  value={state.workerConfiguration}
                  onChange={(value) =>
                    setState((prev) => ({ ...prev, workerConfiguration: value }))
                  }
                  help="Applied to the Kubernetes Job for this run only"
                />
              </Accordion>
            </FlexWrapper>
            <HorizontalDivider />
            <FlexWrapper
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.END}
              gap={8}
              fillWidth
              padding="8px 12px"
            >
              {parsed.error && (
                <Tooltip body={parsed.error} position={TooltipPosition.TOP}>
                  <Chip label="Invalid" icon={WarningIcon} variant={ChipVariant.ERROR} />
                </Tooltip>
              )}
              <Button
                label="Run custom"
                icon={PlayIcon}
                variant={ButtonVariant.PRIMARY}
                size={ButtonSize.SMALL}
                isLoading={isRunning}
                isDisabled={!!parsed.error}
                onClick={() => {
                  onRun(parsed.configuration);
                  close();
                }}
                isIconFilled
              />
            </FlexWrapper>
          </PipelineLayoutNavbarRunButtonDropdown>
        )}
        isIconFilled
      />
    </Tooltip>
  );
};

export default PipelineLayoutNavbarRunButton;
