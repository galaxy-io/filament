import { WarningIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexWrapper, { FlexDirection, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Tooltip, { type TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

const NO_WARNINGS: string[] = [];

interface PipelineTransformFieldsIssuesChipProps {
  issues: string[];
  warnings?: string[];
  position: TooltipPosition;
}

const PipelineTransformFieldsIssuesChip = ({
  issues,
  warnings = NO_WARNINGS,
  position,
}: PipelineTransformFieldsIssuesChipProps) => {
  const messages = issues.length > 0 ? issues : warnings;
  if (messages.length === 0) return null;
  return (
    <Tooltip
      position={position}
      body={
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
          {messages.map((message, index) => (
            // biome-ignore lint/suspicious/noArrayIndexKey: the same message can repeat
            <Text key={index} size={TextSize.BODY_SM}>
              {message}
            </Text>
          ))}
        </FlexWrapper>
      }
    >
      <Chip
        label={issues.length > 0 ? "Invalid" : "Warning"}
        icon={WarningIcon}
        variant={issues.length > 0 ? ChipVariant.ERROR : ChipVariant.WARNING}
        size={ChipSize.SMALL}
      />
    </Tooltip>
  );
};

export default PipelineTransformFieldsIssuesChip;
