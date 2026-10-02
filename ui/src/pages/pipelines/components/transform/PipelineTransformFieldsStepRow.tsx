import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";

import { TRANSFORM_ACTION } from "@/pages/pipelines/components/transform/constants";
import PipelineTransformFieldsIssuesChip from "@/pages/pipelines/components/transform/PipelineTransformFieldsIssuesChip";
import PipelineTransformFieldsStepHeader from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepHeader";
import PipelineTransformFieldsStepSummary from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepSummary";
import PipelineTransformFieldsStepSurface from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepSurface";
import type {
  PipelineTransformFieldsStepHandle,
  TransformStep,
} from "@/pages/pipelines/components/transform/types";

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
        <FlexWrapper alignItems={AlignItems.CENTER} height={TRANSFORM_ACTION} shrink={0}>
          <PipelineTransformFieldsIssuesChip issues={issues} position={TooltipPosition.TOP_END} />
        </FlexWrapper>
      </FlexWrapper>
    </PipelineTransformFieldsStepHeader>
  </PipelineTransformFieldsStepSurface>
);

export default PipelineTransformFieldsStepRow;
