import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";

export enum PipelineCanvasEditMode {
  ADD_NODE = "ADD_NODE",
}

export enum PipelineCanvasInteractionMode {
  GRAB = "GRAB",
  SELECT = "SELECT",
}

export interface PipelineCanvasState {
  nodes: CanvasNode[];
  edges: CanvasEdge[];
  activeMode: PipelineCanvasEditMode | null;
  interactionMode: PipelineCanvasInteractionMode;
  // Y positions nodes were pushed from by overlap resolution, so they return
  // when the space above them frees up. Keyed by node id.
  restoreYs: Record<string, number>;
}
