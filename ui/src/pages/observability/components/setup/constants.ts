import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { CheckCircleIcon, CircleIcon } from "@phosphor-icons/react";

import { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import { TextVariant } from "@galaxy-io/dls/text/Text";

import {
  ObservabilitySetupStep,
  ObservabilitySetupStepStatus,
} from "@/pages/observability/components/setup/types";

export const OBSERVABILITY_SETUP_STEP_ORDER: ObservabilitySetupStep[] = [
  ObservabilitySetupStep.SOURCE,
  ObservabilitySetupStep.SINK,
  ObservabilitySetupStep.PIPELINE,
];

export const OBSERVABILITY_SETUP_STEP_COUNT = OBSERVABILITY_SETUP_STEP_ORDER.length;

export const OBSERVABILITY_SETUP_STEP_TO_TITLE_MAP: Record<ObservabilitySetupStep, string> = {
  [ObservabilitySetupStep.SOURCE]: "Connect a source",
  [ObservabilitySetupStep.SINK]: "Connect a sink",
  [ObservabilitySetupStep.PIPELINE]: "Create a pipeline",
};

export const OBSERVABILITY_SETUP_STEP_TO_DESCRIPTION_MAP: Record<ObservabilitySetupStep, string> = {
  [ObservabilitySetupStep.SOURCE]: "Configure a source system to ingest from",
  [ObservabilitySetupStep.SINK]: "Configure a destination to send data to",
  [ObservabilitySetupStep.PIPELINE]: "Connect your source to your sink and replicate data",
};

export const OBSERVABILITY_SETUP_STEP_TO_ACTION_LABEL_MAP: Record<ObservabilitySetupStep, string> =
  {
    [ObservabilitySetupStep.SOURCE]: "New source",
    [ObservabilitySetupStep.SINK]: "New sink",
    [ObservabilitySetupStep.PIPELINE]: "New pipeline",
  };

export const OBSERVABILITY_SETUP_STATUS_TO_ICON_MAP: Record<
  ObservabilitySetupStepStatus,
  PhosphorIcon
> = {
  [ObservabilitySetupStepStatus.COMPLETED]: CheckCircleIcon,
  [ObservabilitySetupStepStatus.ACTIVE]: CircleIcon,
  [ObservabilitySetupStepStatus.UPCOMING]: CircleIcon,
};

export const OBSERVABILITY_SETUP_STATUS_TO_ICON_WEIGHT_MAP: Record<
  ObservabilitySetupStepStatus,
  IconWeight
> = {
  [ObservabilitySetupStepStatus.COMPLETED]: IconWeight.FILL,
  [ObservabilitySetupStepStatus.ACTIVE]: IconWeight.BOLD,
  [ObservabilitySetupStepStatus.UPCOMING]: IconWeight.REGULAR,
};

export const OBSERVABILITY_SETUP_STATUS_TO_ICON_VARIANT_MAP: Record<
  ObservabilitySetupStepStatus,
  IconVariant
> = {
  [ObservabilitySetupStepStatus.COMPLETED]: IconVariant.SUCCESS,
  [ObservabilitySetupStepStatus.ACTIVE]: IconVariant.PRIMARY,
  [ObservabilitySetupStepStatus.UPCOMING]: IconVariant.TERTIARY,
};

export const OBSERVABILITY_SETUP_STATUS_TO_TITLE_VARIANT_MAP: Record<
  ObservabilitySetupStepStatus,
  TextVariant
> = {
  [ObservabilitySetupStepStatus.COMPLETED]: TextVariant.SECONDARY,
  [ObservabilitySetupStepStatus.ACTIVE]: TextVariant.PRIMARY,
  [ObservabilitySetupStepStatus.UPCOMING]: TextVariant.PRIMARY,
};

export const OBSERVABILITY_SETUP_STATUS_TO_BUTTON_VARIANT_MAP: Record<
  ObservabilitySetupStepStatus,
  ButtonVariant
> = {
  [ObservabilitySetupStepStatus.COMPLETED]: ButtonVariant.TERTIARY,
  [ObservabilitySetupStepStatus.ACTIVE]: ButtonVariant.PRIMARY,
  [ObservabilitySetupStepStatus.UPCOMING]: ButtonVariant.SECONDARY,
};

export const OBSERVABILITY_SETUP_STATUS_TO_OPACITY_MAP: Record<
  ObservabilitySetupStepStatus,
  number
> = {
  [ObservabilitySetupStepStatus.COMPLETED]: 1,
  [ObservabilitySetupStepStatus.ACTIVE]: 1,
  [ObservabilitySetupStepStatus.UPCOMING]: 0.6,
};

export const OBSERVABILITY_SETUP_STATUS_TO_DESCRIPTION_VARIANT_MAP: Record<
  ObservabilitySetupStepStatus,
  TextVariant
> = {
  [ObservabilitySetupStepStatus.COMPLETED]: TextVariant.DISABLED,
  [ObservabilitySetupStepStatus.ACTIVE]: TextVariant.TERTIARY,
  [ObservabilitySetupStepStatus.UPCOMING]: TextVariant.TERTIARY,
};

export const OBSERVABILITY_SETUP_CONTENT_MAX_WIDTH = 600;
