import { FunctionIcon } from "@phosphor-icons/react";

import {
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_HEADER,
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_MESSAGE,
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_HEADER,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import PipelineTransformFieldsPending from "@/pages/pipelines/components/transform/PipelineTransformFieldsPending";

const PipelineCanvasPanelResourceTransformSectionPending = () => (
  <PipelineCanvasPanelSection
    icon={FunctionIcon}
    header={PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_HEADER}
    isEmpty={false}
    emptyHeader={PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_HEADER}
    emptyMessage={PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_MESSAGE}
  >
    <PipelineTransformFieldsPending />
  </PipelineCanvasPanelSection>
);

export default PipelineCanvasPanelResourceTransformSectionPending;
