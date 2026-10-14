import type { FC, PropsWithChildren } from "react";

import ResizablePanels, { ResizablePanel } from "@galaxy-io/dls/layout/ResizablePanels";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import {
  PIPELINE_CANVAS_PANEL_MAIN_ID,
  PIPELINE_CANVAS_PANEL_MAIN_MIN_SIZE,
  PIPELINE_CANVAS_PANEL_SIDEBAR_ID,
  PIPELINE_CANVAS_PANEL_SIDEBAR_MAX_WIDTH,
  PIPELINE_CANVAS_PANEL_SIDEBAR_MIN_WIDTH,
  PIPELINE_CANVAS_PANEL_SIDEBAR_STORAGE_KEY,
  PIPELINE_CANVAS_PANEL_SIDEBAR_WIDTH,
} from "@/pages/pipelines/canvas/panel/constants";
import PipelineCanvasPanel from "@/pages/pipelines/canvas/panel/PipelineCanvasPanel";
import PipelineCanvasPanelColumn from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelColumn";

const PipelineCanvasPanelLayout: FC<PropsWithChildren> = ({ children }) => {
  const { showPanel } = usePipelineCanvasSelection();

  return (
    <ResizablePanels hasDividers storageKey={PIPELINE_CANVAS_PANEL_SIDEBAR_STORAGE_KEY}>
      <ResizablePanel id={PIPELINE_CANVAS_PANEL_MAIN_ID} min={PIPELINE_CANVAS_PANEL_MAIN_MIN_SIZE}>
        <PipelineCanvasPanelColumn>{children}</PipelineCanvasPanelColumn>
      </ResizablePanel>
      {showPanel && (
        <ResizablePanel
          id={PIPELINE_CANVAS_PANEL_SIDEBAR_ID}
          ariaLabel="Configuration"
          defaultValue={PIPELINE_CANVAS_PANEL_SIDEBAR_WIDTH}
          min={PIPELINE_CANVAS_PANEL_SIDEBAR_MIN_WIDTH}
          max={PIPELINE_CANVAS_PANEL_SIDEBAR_MAX_WIDTH}
        >
          <PipelineCanvasPanel />
        </ResizablePanel>
      )}
    </ResizablePanels>
  );
};

export default PipelineCanvasPanelLayout;
