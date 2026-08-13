import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";

export enum PipelineCanvasEditMode {
  ADD_NODE = "ADD_NODE",
}

export interface PipelineCanvasGraph {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
}

export interface PipelineCanvasState {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  activeMode: PipelineCanvasEditMode | null;
}
