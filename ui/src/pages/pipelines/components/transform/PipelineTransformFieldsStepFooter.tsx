import type { FC } from "react";

import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import PipelineTransformFieldsIssuesChip from "@/pages/pipelines/components/transform/PipelineTransformFieldsIssuesChip";

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

const PipelineTransformFieldsStepFooter: FC<PipelineTransformFieldsStepFooterProps> = ({
  issues,
  warnings,
  typeSummary,
  isSaveDisabled,
  isDisabled,
  onSave,
  onCancel,
  onDelete,
}) => (
  <Flex
    alignItems={AlignItems.CENTER}
    justifyContent={JustifyContent.SPACE_BETWEEN}
    gap={8}
    fillWidth
  >
    <Flex alignItems={AlignItems.CENTER} gap={8} grow={1} minWidth={0}>
      <PipelineTransformFieldsIssuesChip issues={issues} warnings={warnings} />
      {issues.length === 0 && typeSummary !== "" && (
        <Flex alignItems={AlignItems.CENTER} gap={4} minWidth={0}>
          <FlexItem shrink={0}>
            <Text size={TextSize.CAPTION} variant={TextVariant.PRIMARY}>
              Output type
            </Text>
          </FlexItem>
          <Text
            size={TextSize.CAPTION}
            variant={TextVariant.SECONDARY}
            family={FontFamily.MONO}
            lineClamp={1}
          >
            {typeSummary}
          </Text>
        </Flex>
      )}
    </Flex>
    <Flex alignItems={AlignItems.CENTER} gap={8} shrink={0}>
      {onDelete && (
        <Button
          icon={TrashIcon}
          variant={ButtonVariant.SECONDARY}
          size={ButtonSize.SMALL}
          onClick={onDelete}
          isDisabled={isDisabled}
          ariaLabel="Delete step"
          tooltip="Delete step"
        />
      )}
      <Button
        label="Cancel"
        variant={ButtonVariant.SECONDARY}
        size={ButtonSize.SMALL}
        onClick={onCancel}
        isDisabled={isDisabled}
      />
      <Button label="Save" size={ButtonSize.SMALL} onClick={onSave} isDisabled={isSaveDisabled} />
    </Flex>
  </Flex>
);

export default PipelineTransformFieldsStepFooter;
