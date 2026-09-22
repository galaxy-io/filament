import { useSortable } from "@dnd-kit/sortable";
import { styled } from "@linaria/react";

import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import {
  usePipelineTransformFieldsActions,
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsStepCard from "@/components/transform/PipelineTransformFieldsStepCard";
import PipelineTransformFieldsStepRow from "@/components/transform/PipelineTransformFieldsStepRow";
import { type TransformStep, TransformStepKind } from "@/components/transform/types";

const Slot = withTheme(styled.div<
  PropsWithTheme<{ $transform: string; $transition: string; $isDragging: boolean }>
>`
  position: relative;
  z-index: ${({ $isDragging }) => ($isDragging ? 1 : "auto")};
  width: 100%;
  min-width: 0;

  background-color: ${({ theme, $isDragging }) =>
    $isDragging ? theme.color.background.secondary : "transparent"};
  transform: ${({ $transform }) => $transform};
  transition: ${({ $transition }) => $transition};
`);

interface PipelineTransformFieldsStepItemProps {
  step: TransformStep;
  issues: string[];
  hasDivider: boolean;
}

const PipelineTransformFieldsStepItem = ({
  step,
  issues,
  hasDivider,
}: PipelineTransformFieldsStepItemProps) => {
  const { draft } = usePipelineTransformFieldsState();
  const { openStep } = usePipelineTransformFieldsActions();
  const { isReadOnly } = usePipelineTransformFieldsEnvironment();
  const {
    setNodeRef,
    setActivatorNodeRef,
    attributes,
    listeners,
    transform,
    transition,
    isDragging,
  } = useSortable({ id: step.id });
  const handle = { ref: setActivatorNodeRef, attributes, listeners };
  const canOpen = !isReadOnly && step.kind !== TransformStepKind.RAW;

  return (
    <Slot
      ref={setNodeRef}
      $transform={transform ? `translate3d(0, ${Math.round(transform.y)}px, 0)` : "none"}
      $transition={transition ?? "none"}
      $isDragging={isDragging}
    >
      {hasDivider && <HorizontalDivider />}
      {draft?.id === step.id ? (
        <PipelineTransformFieldsStepCard draft={draft} handle={handle} />
      ) : (
        <PipelineTransformFieldsStepRow
          step={step}
          issues={issues}
          handle={handle}
          onOpen={canOpen ? () => openStep(step.id) : undefined}
        />
      )}
    </Slot>
  );
};

export default PipelineTransformFieldsStepItem;
