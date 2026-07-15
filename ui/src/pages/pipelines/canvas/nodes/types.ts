import type { NodeProps } from "@xyflow/react";

import type {
  PipelineNodeSourceData,
  PipelineNodeSinkData,
} from "@/pages/pipelines/canvas/types";

export type PipelineNodeSourceProps = NodeProps & {
  data: PipelineNodeSourceData;
};

export type PipelineNodeSinkProps = NodeProps & {
  data: PipelineNodeSinkData;
};
