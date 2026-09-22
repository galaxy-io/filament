import { TrashIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

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

const PipelineTransformFieldsStepFooter = ({
  issues,
  warnings,
  typeSummary,
  isSaveDisabled,
  isDisabled,
  onSave,
  onCancel,
  onDelete,
}: PipelineTransformFieldsStepFooterProps) => (
  <FlexWrapper
    alignItems={AlignItems.CENTER}
    justifyContent={JustifyContent.SPACE_BETWEEN}
    gap={FlexGap.SMALL}
    fillWidth
  >
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL} grow={1} minWidth={0}>
      <PipelineTransformFieldsIssuesChip
        issues={issues}
        warnings={warnings}
        position={TooltipPosition.TOP_START}
      />
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
      <Button label="Save" size={ButtonSize.MEDIUM} onClick={onSave} isDisabled={isSaveDisabled} />
    </FlexWrapper>
  </FlexWrapper>
);

export default PipelineTransformFieldsStepFooter;
