import type { NodeProps } from "@xyflow/react";

import type {
  PipelineNodePlaceholder,
  PipelineNodeSink,
  PipelineNodeSource,
} from "@/pages/pipelines/canvas/types";

export type PipelineNodeSourceProps = NodeProps<PipelineNodeSource>;
export type PipelineNodeSinkProps = NodeProps<PipelineNodeSink>;
export type PipelineNodePlaceholderProps = NodeProps<PipelineNodePlaceholder>;
