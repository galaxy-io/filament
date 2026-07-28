import type { JsonValue } from "@bufbuild/protobuf";
import type { Connection, EdgeChange, NodeChange } from "@xyflow/react";

import type { RunBinding } from "@/gen/ingestion/v1/runs_pb";

import type {
  CanvasEdge,
  CanvasNode,
  PipelineCanvasEditMode,
  PipelineCanvasInteractionMode,
} from "@/pages/pipelines/canvas/types";

export enum PipelineCanvasActionType {
  ADD_NODE = "ADD_NODE",
  REMOVE_NODE = "REMOVE_NODE",
  SET_NODES = "SET_NODES",
  APPLY_NODE_CHANGES = "APPLY_NODE_CHANGES",
  APPLY_EDGE_CHANGES = "APPLY_EDGE_CHANGES",
  CONNECT = "CONNECT",
  SET_ACTIVE_MODE = "SET_ACTIVE_MODE",
  SET_INTERACTION_MODE = "SET_INTERACTION_MODE",
  SET_ACTIVITY_OPEN = "SET_ACTIVITY_OPEN",
  SET_NODE_CONFIG = "SET_NODE_CONFIG",
  SET_RUN_BINDINGS = "SET_RUN_BINDINGS",
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

export interface SetActivityOpenAction {
  type: PipelineCanvasActionType.SET_ACTIVITY_OPEN;
  payload: boolean;
}

export interface SetNodeConfigAction {
  type: PipelineCanvasActionType.SET_NODE_CONFIG;
  payload: { nodeId: string; config: Record<string, JsonValue> };
}

export interface SetRunBindingsAction {
  type: PipelineCanvasActionType.SET_RUN_BINDINGS;
  payload: RunBinding[];
}

export type PipelineCanvasAction =
  | AddNodeAction
  | RemoveNodeAction
  | SetNodesAction
  | ApplyNodeChangesAction
  | ApplyEdgeChangesAction
  | ConnectAction
  | SetActiveModeAction
  | SetInteractionModeAction
  | SetActivityOpenAction
  | SetNodeConfigAction
  | SetRunBindingsAction;
