import {
  closestCenter,
  DndContext,
  type DragEndEvent,
  KeyboardSensor,
  PointerSensor,
  useSensor,
  useSensors,
} from "@dnd-kit/core";
import {
  SortableContext,
  sortableKeyboardCoordinates,
  verticalListSortingStrategy,
} from "@dnd-kit/sortable";

import FlexWrapper from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import type { Resource } from "@/gen/ingestion/v1/connectors_pb";

import {
  TRANSFORM_BODY_INSET,
  TRANSFORM_DRAG_DISTANCE,
  TRANSFORM_GAP,
  TRANSFORM_HEADER_PADDING_X,
} from "@/pages/pipelines/components/transform/constants";
import { usePipelineTransformFieldsIssues } from "@/pages/pipelines/components/transform/hooks/usePipelineTransformFieldsIssues";
import {
  usePipelineTransformFieldsActions,
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsStepItem from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepItem";

const NO_ISSUES: string[] = [];

interface PipelineTransformFieldsResourceStepsProps {
  resource: Resource["name"];
  hasDivider: boolean;
}

const PipelineTransformFieldsResourceSteps = ({
  resource,
  hasDivider,
}: PipelineTransformFieldsResourceStepsProps) => {
  const { stepsByResource, draft } = usePipelineTransformFieldsState();
  const { moveStep } = usePipelineTransformFieldsActions();
  const { resources, isReadOnly } = usePipelineTransformFieldsEnvironment();
  const issuesByStep = usePipelineTransformFieldsIssues(resource);
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: TRANSFORM_DRAG_DISTANCE } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const steps = stepsByResource.get(resource) ?? [];
  const ids = steps.map((step) => step.id);

  const handleDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) return;
    moveStep(resource, ids.indexOf(String(active.id)), ids.indexOf(String(over.id)));
  };

  if (steps.length === 0) return null;

  return (
    <>
      {resources.length > 1 && (
        <FlexWrapper
          padding={`${TRANSFORM_GAP}px ${TRANSFORM_HEADER_PADDING_X}px 0 ${TRANSFORM_BODY_INSET}px`}
        >
          <Text size={TextSize.CAPTION} variant={TextVariant.SECONDARY} isMonospace>
            {resource}
          </Text>
        </FlexWrapper>
      )}
      <DndContext sensors={sensors} collisionDetection={closestCenter} onDragEnd={handleDragEnd}>
        <SortableContext
          items={ids}
          strategy={verticalListSortingStrategy}
          disabled={isReadOnly || draft !== null}
        >
          {steps.map((step, index) => (
            <PipelineTransformFieldsStepItem
              key={step.id}
              step={step}
              issues={issuesByStep.get(step.id) ?? NO_ISSUES}
              hasDivider={hasDivider || index > 0}
            />
          ))}
        </SortableContext>
      </DndContext>
    </>
  );
};

export default PipelineTransformFieldsResourceSteps;
