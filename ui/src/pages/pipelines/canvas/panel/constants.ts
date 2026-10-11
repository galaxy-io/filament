import type { Space } from "@galaxy-io/dls/theme/enums";

import { PipelineCanvasNodeType } from "@/pages/pipelines/canvas/types";

export const PIPELINE_CANVAS_PANEL_WIDTH = 540;
export const PIPELINE_CANVAS_PANEL_COLLAPSED_WIDTH = 28;
export const PIPELINE_CANVAS_PANEL_INSET = 16;
export const PIPELINE_CANVAS_PANEL_TABS_INSET: Space = 12;

export const PIPELINE_CANVAS_NODE_TYPE_TO_RESOURCE_EMPTY_MESSAGE_MAP: Record<
  PipelineCanvasNodeType,
  string
> = {
  [PipelineCanvasNodeType.SOURCE]: "Connect this source to a sink to route resources.",
  [PipelineCanvasNodeType.SINK]: "Connect a source to this sink to route resources.",
  [PipelineCanvasNodeType.PLACEHOLDER]: "Connect a source to a sink to route resources.",
};
