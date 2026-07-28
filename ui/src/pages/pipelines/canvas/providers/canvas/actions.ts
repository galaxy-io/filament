import type { Connection, EdgeChange, NodeChange } from "@xyflow/react";

import type {
  PipelineCanvasEditMode,
  PipelineCanvasInteractionMode,
} from "@/pages/pipelines/canvas/providers/canvas/types";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";

export enum PipelineCanvasActionType {
  ADD_NODE = "ADD_NODE",
  REMOVE_NODE = "REMOVE_NODE",
  SET_NODES = "SET_NODES",
  APPLY_NODE_CHANGES = "APPLY_NODE_CHANGES",
  APPLY_EDGE_CHANGES = "APPLY_EDGE_CHANGES",
  CONNECT = "CONNECT",
  SET_ACTIVE_MODE = "SET_ACTIVE_MODE",
  SET_INTERACTION_MODE = "SET_INTERACTION_MODE",
}

export interface AddNodeAction {
  type: PipelineCanvasActionType.ADD_NODE;
  payload: CanvasNode;
}

export interface RemoveNodeAction {
  type: PipelineCanvasActionType.REMOVE_NODE;
  payload: string;
}

export interface SetNodesAction {
  type: PipelineCanvasActionType.SET_NODES;
  payload: CanvasNode[];
}

export interface ApplyNodeChangesAction {
  type: PipelineCanvasActionType.APPLY_NODE_CHANGES;
  payload: NodeChange<CanvasNode>[];
}

export interface ApplyEdgeChangesAction {
  type: PipelineCanvasActionType.APPLY_EDGE_CHANGES;
  payload: EdgeChange<CanvasEdge>[];
}

export interface ConnectAction {
  type: PipelineCanvasActionType.CONNECT;
  payload: Connection;
}

export interface SetActiveModeAction {
  type: PipelineCanvasActionType.SET_ACTIVE_MODE;
  payload: PipelineCanvasEditMode | null;
}

export interface SetInteractionModeAction {
  type: PipelineCanvasActionType.SET_INTERACTION_MODE;
  payload: PipelineCanvasInteractionMode;
}

export type PipelineCanvasAction =
  | AddNodeAction
  | RemoveNodeAction
  | SetNodesAction
  | ApplyNodeChangesAction
  | ApplyEdgeChangesAction
  | ConnectAction
  | SetActiveModeAction
  | SetInteractionModeAction;
