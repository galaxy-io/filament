import type { FC } from "react";

import GridBackground, {
  GRID_BACKGROUND_OPACITY_VAR,
  GridBackgroundSize,
} from "@galaxy-io/dls/backgrounds/GridBackground";
import GalaxyFilamentWordmark from "@galaxy-io/dls/brand/GalaxyFilamentWordmark";
import ProgressBar, { ProgressBarVariant } from "@galaxy-io/dls/feedback/ProgressBar";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import {
  OBSERVABILITY_SETUP_CONTENT_MAX_WIDTH,
  OBSERVABILITY_SETUP_GRID_OPACITY,
  OBSERVABILITY_SETUP_STEP_COUNT,
  OBSERVABILITY_SETUP_STEP_ORDER,
  OBSERVABILITY_SETUP_STEP_TO_FLOW_MAP,
} from "@/pages/observability/components/setup/constants";
import ObservabilitySetupChecklistStep from "@/pages/observability/components/setup/ObservabilitySetupChecklistStep";
import {
  type ObservabilitySetupStep,
  ObservabilitySetupStepStatus,
} from "@/pages/observability/components/setup/types";
import { useObservabilitySetup } from "@/pages/observability/hooks/useObservabilitySetup";

import { useFilamentFlowOpen } from "@/module/hooks";

const ObservabilitySetupChecklist: FC = () => {
  const openFlow = useFilamentFlowOpen();
  const { completedSteps, activeStep, completedCount } = useObservabilitySetup();

  const handleStepClick = (step: ObservabilitySetupStep) => {
    const { flow, connectorKind } = OBSERVABILITY_SETUP_STEP_TO_FLOW_MAP[step];
    openFlow(flow, connectorKind);
  };

  const getStepStatus = (step: ObservabilitySetupStep) => {
    if (completedSteps.has(step)) {
      return ObservabilitySetupStepStatus.COMPLETED;
    }
    if (step === activeStep) {
      return ObservabilitySetupStepStatus.ACTIVE;
    }
    return ObservabilitySetupStepStatus.UPCOMING;
  };

  return (
    <Box variant={BoxVariant.PRIMARY} height="100%" fillWidth>
      <GridBackground
        size={GridBackgroundSize.X_SMALL}
        style={{ [GRID_BACKGROUND_OPACITY_VAR]: OBSERVABILITY_SETUP_GRID_OPACITY }}
      >
        <Flex
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.CENTER}
          gap={16}
          fillWidth
          maxWidth={OBSERVABILITY_SETUP_CONTENT_MAX_WIDTH}
        >
          <GalaxyFilamentWordmark size={28} />
          <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.CENTER} gap={4} fillWidth>
            <Text size={TextSize.HEADING_SM}>Let's set up your first pipeline</Text>
            <Text variant={TextVariant.SECONDARY}>Three steps to start moving data.</Text>
          </Flex>
          <FlexItem fillWidth>
            <Widget
              variant={WidgetVariant.SECONDARY}
              isFlush
              gap={0}
              header={
                <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
                  {completedCount} of {OBSERVABILITY_SETUP_STEP_COUNT} complete
                </Text>
              }
            >
              {OBSERVABILITY_SETUP_STEP_ORDER.map((step) => (
                <ObservabilitySetupChecklistStep
                  key={step}
                  step={step}
                  status={getStepStatus(step)}
                  onClick={handleStepClick}
                />
              ))}
            </Widget>
          </FlexItem>
          <ProgressBar
            ariaLabel="Setup progress"
            value={(completedCount / OBSERVABILITY_SETUP_STEP_COUNT) * 100}
            variant={ProgressBarVariant.SUCCESS}
          />
        </Flex>
      </GridBackground>
    </Box>
  );
};

export default ObservabilitySetupChecklist;
