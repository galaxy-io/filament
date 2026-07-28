import type { NodeProps } from "@xyflow/react";

import type {
  PipelinePlaceholderNode,
  PipelineSinkNode,
  PipelineSourceNode,
} from "@/pages/pipelines/canvas/types";

export type PipelineNodeSourceProps = NodeProps<PipelineSourceNode>;
export type PipelineNodeSinkProps = NodeProps<PipelineSinkNode>;
export type PipelineNodePlaceholderProps = NodeProps<PipelinePlaceholderNode>;
