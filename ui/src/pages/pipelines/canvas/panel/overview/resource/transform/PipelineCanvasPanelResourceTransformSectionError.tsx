import type { FC } from "react";

import { FunctionIcon } from "@phosphor-icons/react";
import { useQueryErrorResetBoundary } from "@tanstack/react-query";
import type { ErrorComponentProps } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Box from "@galaxy-io/dls/layout/Box";
import ErrorLayout, { ErrorLayoutSize } from "@galaxy-io/dls/layout/ErrorLayout";

import {
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_HEADER,
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_EMPTY_MESSAGE,
  PIPELINE_CANVAS_PANEL_RESOURCE_TRANSFORM_HEADER,
} from "@/pages/pipelines/canvas/panel/overview/resource/transform/constants";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";

import { IS_DEBUG } from "@/constants";

const PipelineCanvasPanelResourceTransformSectionError: FC<ErrorComponentProps> = ({
  error,
  reset,
}) => {
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
    >
      <Box padding={24}>
        <ErrorLayout
          size={ErrorLayoutSize.SMALL}
          header="Unable to load transformations"
          description="The function catalog could not be loaded."
          detail={IS_DEBUG ? error.message : undefined}
          actions={
            <Button label="Try again" variant={ButtonVariant.SECONDARY} onClick={handleRetry} />
          }
        />
      </Box>
    </PipelineCanvasPanelSection>
  );
};

export default PipelineCanvasPanelResourceTransformSectionError;
