import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";
import { match } from "ts-pattern";

import DotGridBackground from "@galaxy-io/dls/backgrounds/DotGridBackground";
import { ButtonSize } from "@galaxy-io/dls/buttons/Button";
import ProgressBar, { ProgressBarVariant } from "@galaxy-io/dls/charts/ProgressBar";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import GalaxyFilamentWordmark from "@galaxy-io/dls/icons/GalaxyFilamentWordmark";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { useGalaxyTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import DocsButton from "@/components/DocsButton";

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

import { Flow } from "@/routes/__root";

const SetupContent = styled.div`
  position: relative;

  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 20px;

  width: 100%;
  max-width: ${OBSERVABILITY_SETUP_CONTENT_MAX_WIDTH}px;
`;

const SetupCard = withTheme(styled.div<PropsWithTheme>`
  display: flex;
  flex-direction: column;

  width: 100%;

  background-color: ${({ theme }) => theme.color.background.secondary};

  border: 0.5px solid ${({ theme }) => theme.color.border.galaxy};
  border-radius: 6px;

  overflow: hidden;
`);

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
    <DotGridBackground dotSize={2} backgroundColor={theme.color.background.primary}>
      <SetupContent>
        <GalaxyFilamentWordmark height={28} />
        <FlexWrapper
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.CENTER}
          gap={4}
          fillWidth
        >
          <Text size={TextSize.HEADING_SM}>Let's set up your first pipeline</Text>
          <Text variant={TextVariant.SECONDARY}>Three steps to complete your onboarding</Text>
        </FlexWrapper>
        <SetupCard>
          <FlexWrapper padding={"12px 16px"}>
            <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
              {completedCount} of {OBSERVABILITY_SETUP_STEP_COUNT} complete
            </Text>
          </FlexWrapper>
          <HorizontalDivider />
          {OBSERVABILITY_SETUP_STEP_ORDER.map((step) => (
            <ObservabilitySetupChecklistStep
              key={step}
              step={step}
              status={getStepStatus(step)}
              onClick={handleStepClick}
            />
          ))}
        </SetupCard>
        <ProgressBar
          percentage={(completedCount / OBSERVABILITY_SETUP_STEP_COUNT) * 100}
          variant={ProgressBarVariant.PRIMARY}
          height={4}
          noAnimation
        />
        <DocsButton label="Read the docs" size={ButtonSize.LARGE} />
      </SetupContent>
    </DotGridBackground>
  );
};

export default ObservabilitySetupChecklist;
