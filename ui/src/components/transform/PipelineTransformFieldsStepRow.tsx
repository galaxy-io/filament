import { WarningIcon } from "@phosphor-icons/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Tooltip, { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import PipelineTransformFieldsStepHeader, {
  PipelineTransformFieldsStepSurface,
} from "@/components/transform/PipelineTransformFieldsStepHeader";
import PipelineTransformFieldsStepSummary from "@/components/transform/PipelineTransformFieldsStepSummary";
import type {
  PipelineTransformFieldsStepHandle,
  TransformStep,
} from "@/components/transform/types";

interface PipelineTransformFieldsStepRowProps {
  step: TransformStep;
  issues: string[];
  handle: PipelineTransformFieldsStepHandle;
  onOpen?: () => void;
}

const PipelineTransformFieldsStepRow = ({
  step,
  issues,
  handle,
  onOpen,
}: PipelineTransformFieldsStepRowProps) => (
  <PipelineTransformFieldsStepSurface $isHoverable={onOpen !== undefined}>
    <PipelineTransformFieldsStepHeader isOpen={false} onToggle={onOpen} handle={handle}>
      <FlexWrapper alignItems={AlignItems.START} gap={FlexGap.SMALL} fillWidth>
        <FlexItem grow={1} minWidth={0}>
          <PipelineTransformFieldsStepSummary step={step} />
        </FlexItem>
        {issues.length > 0 && (
          <FlexItem shrink={0}>
            <Tooltip
              position={TooltipPosition.TOP_END}
              body={
                <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XSMALL}>
                  {issues.map((message, index) => (
                    // biome-ignore lint/suspicious/noArrayIndexKey: the same message can repeat
                    <Text key={index} size={TextSize.BODY_SM}>
                      {message}
                    </Text>
                  ))}
                </FlexWrapper>
              }
            >
              <Chip
                label="Invalid"
                icon={WarningIcon}
                variant={ChipVariant.ERROR}
                size={ChipSize.SMALL}
              />
            </Tooltip>
          </FlexItem>
        )}
      </FlexWrapper>
    </PipelineTransformFieldsStepHeader>
  </PipelineTransformFieldsStepSurface>
);

export default PipelineTransformFieldsStepRow;
