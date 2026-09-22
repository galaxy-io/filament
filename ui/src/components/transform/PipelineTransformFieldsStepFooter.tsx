import { TrashIcon, WarningIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

interface PipelineTransformFieldsStepFooterProps {
  issues: string[];
  warnings: string[];
  typeSummary: string;
  isSaveDisabled: boolean;
  isDisabled: boolean;
  onSave: () => void;
  onCancel: () => void;
  onDelete?: () => void;
}

const PipelineTransformFieldsStepFooter = ({
  issues,
  warnings,
  typeSummary,
  isSaveDisabled,
  isDisabled,
  onSave,
  onCancel,
  onDelete,
}: PipelineTransformFieldsStepFooterProps) => {
  const messages = issues.length > 0 ? issues : warnings;
  return (
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={FlexGap.SMALL}
      fillWidth
    >
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL} grow={1} minWidth={0}>
        {messages.length > 0 && (
          <Tooltip
            position={TooltipPosition.TOP_START}
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
        )}
        {issues.length === 0 && typeSummary !== "" && (
          <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.XSMALL} minWidth={0}>
            <FlexItem shrink={0}>
              <Text size={TextSize.CAPTION} variant={TextVariant.PRIMARY}>
                Output type
              </Text>
            </FlexItem>
            <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isMonospace isEllipsis>
              {typeSummary}
            </Text>
          </FlexWrapper>
        )}
      </FlexWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL} shrink={0}>
        {onDelete && (
          <Button
            icon={TrashIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.MEDIUM}
            onClick={onDelete}
            isDisabled={isDisabled}
            ariaLabel="Delete step"
          />
        )}
        <Button
          label="Cancel"
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.MEDIUM}
          onClick={onCancel}
          isDisabled={isDisabled}
        />
        <Button
          label="Save"
          size={ButtonSize.MEDIUM}
          onClick={onSave}
          isDisabled={isSaveDisabled}
        />
      </FlexWrapper>
    </FlexWrapper>
  );
};

export default PipelineTransformFieldsStepFooter;
