import type { FC } from "react";

import { ArrowRightIcon } from "@phosphor-icons/react";
import { match } from "ts-pattern";

import Button from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Icon from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import {
  OBSERVABILITY_SETUP_STATUS_TO_BUTTON_VARIANT_MAP,
  OBSERVABILITY_SETUP_STATUS_TO_DESCRIPTION_VARIANT_MAP,
  OBSERVABILITY_SETUP_STATUS_TO_ICON_MAP,
  OBSERVABILITY_SETUP_STATUS_TO_ICON_VARIANT_MAP,
  OBSERVABILITY_SETUP_STATUS_TO_ICON_WEIGHT_MAP,
  OBSERVABILITY_SETUP_STATUS_TO_TITLE_VARIANT_MAP,
  OBSERVABILITY_SETUP_STEP_TO_ACTION_LABEL_MAP,
  OBSERVABILITY_SETUP_STEP_TO_DESCRIPTION_MAP,
  OBSERVABILITY_SETUP_STEP_TO_TITLE_MAP,
} from "@/pages/observability/components/setup/constants";
import type { ObservabilitySetupStep } from "@/pages/observability/components/setup/types";
import { ObservabilitySetupStepStatus } from "@/pages/observability/components/setup/types";

const OBSERVABILITY_SETUP_STEP_MARKER_SIZE = 18;
const OBSERVABILITY_SETUP_STEP_MARKER_HEIGHT = 20;

interface ObservabilitySetupChecklistStepProps {
  step: ObservabilitySetupStep;
  status: ObservabilitySetupStepStatus;
  onClick: (step: ObservabilitySetupStep) => void;
}

const ObservabilitySetupChecklistStep: FC<ObservabilitySetupChecklistStepProps> = ({
  step,
  status,
  onClick,
}) => {
  const handleClick = () => onClick(step);

  return (
    <Flex alignItems={AlignItems.START} gap={12} padding={[12, 12]} fillWidth>
      <Flex
        alignItems={AlignItems.CENTER}
        shrink={0}
        height={OBSERVABILITY_SETUP_STEP_MARKER_HEIGHT}
      >
        <Icon
          component={OBSERVABILITY_SETUP_STATUS_TO_ICON_MAP[status]}
          size={OBSERVABILITY_SETUP_STEP_MARKER_SIZE}
          weight={OBSERVABILITY_SETUP_STATUS_TO_ICON_WEIGHT_MAP[status]}
          variant={OBSERVABILITY_SETUP_STATUS_TO_ICON_VARIANT_MAP[status]}
        />
      </Flex>
      <FlexItem grow={1} minWidth={0}>
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={2} minWidth={0}>
          <Text
            size={TextSize.BODY_LG}
            variant={OBSERVABILITY_SETUP_STATUS_TO_TITLE_VARIANT_MAP[status]}
          >
            {OBSERVABILITY_SETUP_STEP_TO_TITLE_MAP[step]}
          </Text>
          <Text
            size={TextSize.BODY_SM}
            variant={OBSERVABILITY_SETUP_STATUS_TO_DESCRIPTION_VARIANT_MAP[status]}
          >
            {OBSERVABILITY_SETUP_STEP_TO_DESCRIPTION_MAP[step]}
          </Text>
        </Flex>
      </FlexItem>
      <FlexItem shrink={0}>
        {match(status)
          .with(ObservabilitySetupStepStatus.COMPLETED, () => (
            <Chip label="Done" variant={ChipVariant.SUCCESS} />
          ))
          .otherwise(() => (
            <Button
              label={OBSERVABILITY_SETUP_STEP_TO_ACTION_LABEL_MAP[step]}
              icon={ArrowRightIcon}
              variant={OBSERVABILITY_SETUP_STATUS_TO_BUTTON_VARIANT_MAP[status]}
              onClick={handleClick}
              isIconTrailing
            />
          ))}
      </FlexItem>
    </Flex>
  );
};

export default ObservabilitySetupChecklistStep;
