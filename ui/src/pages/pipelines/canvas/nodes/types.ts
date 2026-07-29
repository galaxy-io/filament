import type { NodeProps } from "@xyflow/react";

import type {
  PipelineCanvasPlaceholderNode,
  PipelineCanvasSinkNode,
  PipelineCanvasSourceNode,
} from "@/pages/pipelines/canvas/types";

export type PipelineCanvasNodeSourceProps = NodeProps<PipelineCanvasSourceNode>;
export type PipelineCanvasNodeSinkProps = NodeProps<PipelineCanvasSinkNode>;
export type PipelineCanvasNodePlaceholderProps = NodeProps<PipelineCanvasPlaceholderNode>;
