import type { FC } from "react";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";

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

const PipelineTransformFieldsStepRow: FC<PipelineTransformFieldsStepRowProps> = ({
  step,
  issues,
  handle,
  onOpen,
}) => (
  <PipelineTransformFieldsStepSurface $isHoverable={onOpen !== undefined}>
    <PipelineTransformFieldsStepHeader isOpen={false} onToggle={onOpen} handle={handle}>
      <Flex alignItems={AlignItems.START} gap={8} fillWidth>
        <FlexItem grow={1} minWidth={0}>
          <PipelineTransformFieldsStepSummary step={step} />
        </FlexItem>
        <Flex alignItems={AlignItems.CENTER} height={TRANSFORM_ACTION} shrink={0}>
          <PipelineTransformFieldsIssuesChip issues={issues} />
        </Flex>
      </Flex>
    </PipelineTransformFieldsStepHeader>
  </PipelineTransformFieldsStepSurface>
);

export default PipelineTransformFieldsStepRow;
