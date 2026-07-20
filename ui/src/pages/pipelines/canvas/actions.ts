import type { Connection, EdgeChange, NodeChange } from "@xyflow/react";

import type {
  PipelineCanvasEditMode,
  PipelineEdge,
  PipelineNode,
} from "@/pages/pipelines/canvas/types";

import type { RunBinding } from "@/gen/ingestion/v1/runs_pb";

export enum PipelineCanvasActionType {
  ADD_NODE = "ADD_NODE",
  REMOVE_NODE = "REMOVE_NODE",
  SET_NODES = "SET_NODES",
  APPLY_NODE_CHANGES = "APPLY_NODE_CHANGES",
  ADD_EDGE = "ADD_EDGE",
  REMOVE_EDGE = "REMOVE_EDGE",
  SET_EDGES = "SET_EDGES",
  APPLY_EDGE_CHANGES = "APPLY_EDGE_CHANGES",
  CONNECT = "CONNECT",
  SET_ACTIVE_MODE = "SET_ACTIVE_MODE",
  SET_ACTIVITY_OPEN = "SET_ACTIVITY_OPEN",
  SET_RUN_BINDINGS = "SET_RUN_BINDINGS",
}

export type PipelineCanvasAction =
  | { type: PipelineCanvasActionType.ADD_NODE; payload: PipelineNode }
  | { type: PipelineCanvasActionType.REMOVE_NODE; payload: string }
  | { type: PipelineCanvasActionType.SET_NODES; payload: PipelineNode[] }
  | { type: PipelineCanvasActionType.APPLY_NODE_CHANGES; payload: NodeChange<PipelineNode>[] }
  | { type: PipelineCanvasActionType.ADD_EDGE; payload: PipelineEdge }
  | { type: PipelineCanvasActionType.REMOVE_EDGE; payload: string }
  | { type: PipelineCanvasActionType.SET_EDGES; payload: PipelineEdge[] }
  | { type: PipelineCanvasActionType.APPLY_EDGE_CHANGES; payload: EdgeChange<PipelineEdge>[] }
  | { type: PipelineCanvasActionType.CONNECT; payload: Connection }
  | { type: PipelineCanvasActionType.SET_ACTIVE_MODE; payload: PipelineCanvasEditMode | null }
  | { type: PipelineCanvasActionType.SET_ACTIVITY_OPEN; payload: boolean }
  | { type: PipelineCanvasActionType.SET_RUN_BINDINGS; payload: RunBinding[] };
