import type {
  PipelineCanvasGraph,
  PipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/types";

export const createInitialPipelineCanvasState = (
  graph: PipelineCanvasGraph,
): PipelineCanvasState => ({
  nodes: graph.nodes,
  edges: graph.edges,
  activeMode: null,
});
