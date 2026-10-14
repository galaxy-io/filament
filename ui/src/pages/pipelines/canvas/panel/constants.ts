import { PipelineCanvasNodeType } from "@/pages/pipelines/canvas/types";

export const PIPELINE_CANVAS_PANEL_MAIN_ID = "pipeline-canvas-main";
export const PIPELINE_CANVAS_PANEL_VIEW_ID = "pipeline-canvas-view";
export const PIPELINE_CANVAS_PANEL_SIDEBAR_ID = "pipeline-canvas-sidebar";
export const PIPELINE_CANVAS_PANEL_ACTIVITY_ID = "pipeline-canvas-activity";

export const PIPELINE_CANVAS_PANEL_MAIN_MIN_SIZE = 240;
export const PIPELINE_CANVAS_PANEL_SIDEBAR_WIDTH = 420;
export const PIPELINE_CANVAS_PANEL_SIDEBAR_MIN_WIDTH = 320;
export const PIPELINE_CANVAS_PANEL_SIDEBAR_MAX_WIDTH = 760;
export const PIPELINE_CANVAS_PANEL_ACTIVITY_HEIGHT = 320;
export const PIPELINE_CANVAS_PANEL_ACTIVITY_MIN_HEIGHT = 160;
export const PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_HEIGHT = 640;

export const PIPELINE_CANVAS_PANEL_SIDEBAR_STORAGE_KEY = "filament:pipeline-canvas:sidebar";
export const PIPELINE_CANVAS_PANEL_ACTIVITY_STORAGE_KEY = "filament:pipeline-canvas:activity";

export const PIPELINE_CANVAS_NODE_TYPE_TO_RESOURCE_EMPTY_MESSAGE_MAP: Record<
  PipelineCanvasNodeType,
  string
> = {
  [PipelineCanvasNodeType.SOURCE]: "Connect this source to a sink to route resources.",
  [PipelineCanvasNodeType.SINK]: "Connect a source to this sink to route resources.",
  [PipelineCanvasNodeType.PLACEHOLDER]: "Connect a source to a sink to route resources.",
};
