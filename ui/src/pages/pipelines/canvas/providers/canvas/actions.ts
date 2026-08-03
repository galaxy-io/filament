import type { JsonValue } from "@bufbuild/protobuf";
import type { Connection, EdgeChange, NodeChange } from "@xyflow/react";

import type { IngestionType } from "@/gen/ingestion/v1/common_pb";

import type {
  PipelineCanvasEditMode,
  PipelineCanvasGraph,
  PipelineCanvasInteractionMode,
} from "@/pages/pipelines/canvas/providers/canvas/types";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";

export enum PipelineCanvasActionType {
  LOAD_GRAPH = "LOAD_GRAPH",
  ADD_NODE = "ADD_NODE",
  REMOVE_NODE = "REMOVE_NODE",
  SET_NODES = "SET_NODES",
  APPLY_NODE_CHANGES = "APPLY_NODE_CHANGES",
  APPLY_EDGE_CHANGES = "APPLY_EDGE_CHANGES",
  CONNECT = "CONNECT",
  SET_ACTIVE_MODE = "SET_ACTIVE_MODE",
  SET_INTERACTION_MODE = "SET_INTERACTION_MODE",
  SET_NODE_CONFIG = "SET_NODE_CONFIG",
  SET_EDGE_INGESTION_TYPE = "SET_EDGE_INGESTION_TYPE",
}

export interface LoadGraphAction {
  type: PipelineCanvasActionType.LOAD_GRAPH;
  payload: PipelineCanvasGraph;
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

export interface SetNodeConfigAction {
  type: PipelineCanvasActionType.SET_NODE_CONFIG;
  payload: { nodeId: string; config: Record<string, JsonValue> };
}

export interface SetEdgeIngestionTypeAction {
  type: PipelineCanvasActionType.SET_EDGE_INGESTION_TYPE;
  payload: { edgeId: string; ingestionType: IngestionType };
}

export type PipelineCanvasAction =
  | LoadGraphAction
  | AddNodeAction
  | RemoveNodeAction
  | SetNodesAction
  | ApplyNodeChangesAction
  | ApplyEdgeChangesAction
  | ConnectAction
  | SetActiveModeAction
  | SetInteractionModeAction
  | SetNodeConfigAction
  | SetEdgeIngestionTypeAction;
