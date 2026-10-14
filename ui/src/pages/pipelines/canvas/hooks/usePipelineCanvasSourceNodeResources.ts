import type { Resource } from "@/gen/ingestion/v1/connectors_pb";

import { usePipelineCanvasSourceResources } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSourceResources";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  PipelineCanvasNodeType,
  type PipelineCanvasSourceNode,
} from "@/pages/pipelines/canvas/types";

export const usePipelineCanvasSourceNodeResources = (): Resource["name"][] => {
  const { nodes } = usePipelineCanvasState();
  const connectionId =
    nodes.find(
      (node): node is PipelineCanvasSourceNode => node.type === PipelineCanvasNodeType.SOURCE,
    )?.data.connectionId ?? "";

  return usePipelineCanvasSourceResources(connectionId).selectableNames;
};
