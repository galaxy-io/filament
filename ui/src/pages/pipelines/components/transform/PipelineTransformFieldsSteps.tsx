import type { FC } from "react";

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

import { TRANSFORM_DRAG_DISTANCE } from "@/pages/pipelines/components/transform/constants";
import { usePipelineTransformFieldsIssues } from "@/pages/pipelines/components/transform/hooks/usePipelineTransformFieldsIssues";
import {
  usePipelineTransformFieldsActions,
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import PipelineTransformFieldsStepItem from "@/pages/pipelines/components/transform/PipelineTransformFieldsStepItem";

const NO_ISSUES: string[] = [];

const PipelineTransformFieldsSteps: FC = () => {
  const { steps, draft } = usePipelineTransformFieldsState();
  const { moveStep } = usePipelineTransformFieldsActions();
  const { isReadOnly } = usePipelineTransformFieldsEnvironment();
  const issuesByStep = usePipelineTransformFieldsIssues();
  const sensors = useSensors(
    useSensor(PointerSensor, { activationConstraint: { distance: TRANSFORM_DRAG_DISTANCE } }),
    useSensor(KeyboardSensor, { coordinateGetter: sortableKeyboardCoordinates }),
  );
  const ids = steps.map((step) => step.id);

  const handleDragEnd = ({ active, over }: DragEndEvent) => {
    if (!over || active.id === over.id) return;
    moveStep(ids.indexOf(String(active.id)), ids.indexOf(String(over.id)));
  };

  if (steps.length === 0) return null;

  return (
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
            hasDivider={index > 0}
          />
        ))}
      </SortableContext>
    </DndContext>
  );
};

export default PipelineTransformFieldsSteps;
