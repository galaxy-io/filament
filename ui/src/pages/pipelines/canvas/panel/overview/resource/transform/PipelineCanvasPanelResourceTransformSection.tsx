import { useState } from "react";

import { FunctionIcon, PlusIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import {
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_HEADER,
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_MESSAGE,
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_HEADER,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import PipelineTransformFields from "@/pages/pipelines/components/transform/PipelineTransformFields";
import {
  usePipelineTransformFieldsActions,
  usePipelineTransformFieldsEnvironment,
  usePipelineTransformFieldsState,
} from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";

const PipelineCanvasPanelResourceTransformSection = () => {
  const { stepsByResource, draft } = usePipelineTransformFieldsState();
  const { openNew } = usePipelineTransformFieldsActions();
  const { isReadOnly } = usePipelineTransformFieldsEnvironment();
  const [isOpen, setIsOpen] = useState(true);
  const hasContent =
    draft !== null || [...stepsByResource.values()].some((steps) => steps.length > 0);

  return (
    <PipelineCanvasPanelSection
      icon={FunctionIcon}
      header={PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_HEADER}
      isEmpty={!hasContent}
      emptyHeader={PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_HEADER}
      emptyMessage={PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_MESSAGE}
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
            isDisabled={draft?.id === null}
          />
        )
      }
    >
      <PipelineTransformFields />
    </PipelineCanvasPanelSection>
  );
};

export default PipelineCanvasPanelResourceTransformSection;
