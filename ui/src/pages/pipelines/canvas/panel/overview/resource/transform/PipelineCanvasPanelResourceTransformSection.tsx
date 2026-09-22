import { useState } from "react";

import { PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import PipelineTransformFields from "@/components/transform/PipelineTransformFields";
import {
  usePipelineTransformFieldsActions,
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/components/transform/PipelineTransformFieldsProvider";

import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";

const PipelineCanvasPanelResourceTransformSection = () => {
  const { stepsByResource, draft } = usePipelineTransformFieldsState();
  const { openNew } = usePipelineTransformFieldsActions();
  const { resources, isReadOnly } = usePipelineTransformFieldsEnvironment();
  const [isOpen, setIsOpen] = useState(true);
  const hasContent =
    draft !== null || [...stepsByResource.values()].some((steps) => steps.length > 0);

  return (
    <PipelineCanvasPanelSection
      header="Transformations"
      isEmpty={!hasContent}
      emptyHeader="No steps"
      emptyMessage="Columns arrive at the sink as they leave the source."
      isOpen={isOpen}
      onToggle={() => setIsOpen((prev) => !prev)}
      headerAction={
        isReadOnly ? undefined : (
          <Button
            label="Add step"
            icon={PlusIcon}
            variant={ButtonVariant.SECONDARY}
            size={ButtonSize.SMALL}
            onClick={() => {
              setIsOpen(true);
              openNew();
            }}
            isDisabled={draft?.id === null || resources.length === 0}
          />
        )
      }
    >
      <PipelineTransformFields />
    </PipelineCanvasPanelSection>
  );
};

export default PipelineCanvasPanelResourceTransformSection;
