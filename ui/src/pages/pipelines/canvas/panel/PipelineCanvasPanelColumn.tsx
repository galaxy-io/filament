import type { FC, PropsWithChildren } from "react";

import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ResizablePanels, { ResizablePanel } from "@galaxy-io/dls/layout/ResizablePanels";
import { Orientation } from "@galaxy-io/dls/theme/enums";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { usePipelineCanvasPanelActivityRunIds } from "@/pages/pipelines/canvas/panel/activity/hooks/usePipelineCanvasPanelActivityRunIds";
import PipelineCanvasPanelActivity from "@/pages/pipelines/canvas/panel/activity/PipelineCanvasPanelActivity";
import PipelineCanvasPanelActivityHeader from "@/pages/pipelines/canvas/panel/activity/PipelineCanvasPanelActivityHeader";
import {
  PIPELINE_CANVAS_PANEL_ACTIVITY_HEIGHT,
  PIPELINE_CANVAS_PANEL_ACTIVITY_ID,
  PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_HEIGHT,
  PIPELINE_CANVAS_PANEL_ACTIVITY_MIN_HEIGHT,
  PIPELINE_CANVAS_PANEL_ACTIVITY_STORAGE_KEY,
  PIPELINE_CANVAS_PANEL_MAIN_MIN_SIZE,
  PIPELINE_CANVAS_PANEL_VIEW_ID,
} from "@/pages/pipelines/canvas/panel/constants";

const PipelineCanvasPanelColumn: FC<PropsWithChildren> = ({ children }) => {
  const { showActivity, setShowActivity } = usePipelineCanvasSelection();
  const runIds = usePipelineCanvasPanelActivityRunIds();

  const header = (
    <PipelineCanvasPanelActivityHeader
      isOpen={showActivity}
      onToggle={() => setShowActivity(!showActivity)}
    />
  );

  return (
    <Flex direction={FlexDirection.COLUMN} height="100%">
      <FlexItem grow={1} minHeight={0}>
        <ResizablePanels
          orientation={Orientation.VERTICAL}
          hasDividers
          storageKey={PIPELINE_CANVAS_PANEL_ACTIVITY_STORAGE_KEY}
        >
          <ResizablePanel
            id={PIPELINE_CANVAS_PANEL_VIEW_ID}
            min={PIPELINE_CANVAS_PANEL_MAIN_MIN_SIZE}
          >
            <Box position="relative" height="100%">
              {children}
            </Box>
          </ResizablePanel>
          {showActivity && (
            <ResizablePanel
              id={PIPELINE_CANVAS_PANEL_ACTIVITY_ID}
              ariaLabel="Activity"
              defaultValue={PIPELINE_CANVAS_PANEL_ACTIVITY_HEIGHT}
              min={PIPELINE_CANVAS_PANEL_ACTIVITY_MIN_HEIGHT}
              max={PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_HEIGHT}
            >
              <Box variant={BoxVariant.PRIMARY} height="100%">
                <Flex direction={FlexDirection.COLUMN} height="100%">
                  {header}
                  <PipelineCanvasPanelActivity runIds={runIds} />
                </Flex>
              </Box>
            </ResizablePanel>
          )}
        </ResizablePanels>
      </FlexItem>
      {!showActivity && (
        <Box variant={BoxVariant.PRIMARY}>
          <Divider />
          {header}
        </Box>
      )}
    </Flex>
  );
};

export default PipelineCanvasPanelColumn;
