import { FunctionIcon } from "@phosphor-icons/react";
import { useQueryErrorResetBoundary } from "@tanstack/react-query";
import type { ErrorComponentProps } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import ErrorLayout from "@/layouts/ErrorLayout";
import { LayoutSize } from "@/layouts/types";

import {
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_HEADER,
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_MESSAGE,
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_HEADER,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";

const PipelineCanvasPanelResourceTransformSectionError = ({
  error,
  reset,
}: ErrorComponentProps) => {
  // The failed query keeps its error until it is reset, so a bare boundary
  // reset would rethrow the same error without a fetch.
  const { reset: resetQueries } = useQueryErrorResetBoundary();

  const handleRetry = () => {
    resetQueries();
    reset();
  };

  return (
    <PipelineCanvasPanelSection
      icon={FunctionIcon}
      header={PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_HEADER}
      isEmpty={false}
      emptyHeader={PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_HEADER}
      emptyMessage={PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_MESSAGE}
      padding="24px"
    >
      <ErrorLayout
        size={LayoutSize.SMALL}
        header="Unable to load transformations"
        message="The function catalog could not be loaded."
        error={error}
        actions={
          <Button label="Try again" variant={ButtonVariant.SECONDARY} onClick={handleRetry} />
        }
      />
    </PipelineCanvasPanelSection>
  );
};

export default PipelineCanvasPanelResourceTransformSectionError;
