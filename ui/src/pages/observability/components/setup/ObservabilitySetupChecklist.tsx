import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";
import { match } from "ts-pattern";

import GridBackground from "@galaxy-io/dls/backgrounds/GridBackground";
import GalaxyFilamentWordmark from "@galaxy-io/dls/brand/GalaxyFilamentWordmark";
import { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import ProgressBar, { ProgressBarVariant } from "@galaxy-io/dls/feedback/ProgressBar";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import { useGalaxyTheme } from "@galaxy-io/dls/theme/useGalaxyTheme";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import DocsButton from "@/components/DocsButton";

import { Flow } from "@/layouts/app/types";

import {
  OBSERVABILITY_SETUP_CONTENT_MAX_WIDTH,
  OBSERVABILITY_SETUP_STEP_COUNT,
  OBSERVABILITY_SETUP_STEP_ORDER,
} from "@/pages/observability/components/setup/constants";
import ObservabilitySetupChecklistStep from "@/pages/observability/components/setup/ObservabilitySetupChecklistStep";
import {
  ObservabilitySetupStep,
  ObservabilitySetupStepStatus,
} from "@/pages/observability/components/setup/types";
import { useObservabilitySetup } from "@/pages/observability/hooks/useObservabilitySetup";

const SetupContent = styled.div`
  position: relative;

  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;

  width: 100%;
  max-width: ${OBSERVABILITY_SETUP_CONTENT_MAX_WIDTH}px;
`;

const SetupCard = styled.div`
  display: flex;
  flex-direction: column;

  width: 100%;

  background-color: ${t.color.background.secondary};

  border: 0.5px solid ${t.color.border.focused};
  border-radius: 6px;

  overflow: hidden;
`;

const ObservabilitySetupChecklist = () => {
  const { theme } = useGalaxyTheme();
  const navigate = useNavigate();
  const { completedSteps, activeStep, completedCount } = useObservabilitySetup();

  const handleStepClick = (step: ObservabilitySetupStep) => {
    match(step)
      .with(ObservabilitySetupStep.SOURCE, () => {
        void navigate({
          to: ".",
          search: (prev) => ({
            ...prev,
            connectionId: undefined,
            flow: Flow.CREATE_CONNECTION,
            connectorKind: ConnectorKind.SOURCE,
          }),
        });
      })
      .with(ObservabilitySetupStep.SINK, () => {
        void navigate({
          to: ".",
          search: (prev) => ({
            ...prev,
            connectionId: undefined,
            flow: Flow.CREATE_CONNECTION,
            connectorKind: ConnectorKind.SINK,
          }),
        });
      })
      .with(ObservabilitySetupStep.PIPELINE, () => {
        void navigate({
          to: ".",
          search: (prev) => ({ ...prev, connectionId: undefined, flow: Flow.CREATE_PIPELINE }),
        });
      })
      .exhaustive();
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
    <GridBackground /* @dls-migrate gridbackground.backgroundColor: Put the grid in a `Box` with the surface `variant` (`<Box variant={BoxVariant.BASE}>` for the 1.x default). */
      backgroundColor={theme.color.background.primary}
    >
      <SetupContent>
        <GalaxyFilamentWordmark size={28} />
        <Flex
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.CENTER}
          /* @dls-migrate layout.off-scale: Pick a value on the space scale (or a CSS-order tuple of them). */ gap={
            6
          }
          fillWidth
        >
          <Text size={TextSize.HEADING_SM}>Let's set up your first pipeline</Text>
          <Text variant={TextVariant.SECONDARY}>Three steps to start moving data.</Text>
        </Flex>
        <SetupCard>
          <Flex alignItems={AlignItems.START} padding={[12, 16]}>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              {completedCount} of {OBSERVABILITY_SETUP_STEP_COUNT} complete
            </Text>
          </Flex>
          <Divider />
          {OBSERVABILITY_SETUP_STEP_ORDER.map((step) => (
            <ObservabilitySetupChecklistStep
              key={step}
              step={step}
              status={getStepStatus(step)}
              onClick={handleStepClick}
            />
          ))}
        </SetupCard>
        <ProgressBar /* @dls-migrate progressbar.ariaLabel: Name the bar: add `label` or `ariaLabel`. */
          value={(completedCount / OBSERVABILITY_SETUP_STEP_COUNT) * 100}
          variant={ProgressBarVariant.SUCCESS}
        />
        <DocsButton label="Read the docs" size={ButtonSize.LARGE} />
      </SetupContent>
    </GridBackground>
  );
};

export default ObservabilitySetupChecklist;
