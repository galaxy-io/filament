import type { BuiltInNode, Edge, Node } from "@xyflow/react";

import type { PipelineCanvasAction } from "@/pages/pipelines/canvas/actions";

import type { RunBinding } from "@/gen/ingestion/v1/runs_pb";

export enum PipelineNodeType {
  SOURCE = "SOURCE",
  SINK = "SINK",
}

export interface PipelineNodeSourceTableInfo {
  name: string;
  rowCount: string;
  isConnected: boolean;
}

export type PipelineNodeSourceData = {
  label: string;
  connector: string;
  connectionId: string;
};

export type PipelineNodeSinkData = {
  label: string;
  connector: string;
  connectionId: string;
};

export type PipelineNodeSource = Node<PipelineNodeSourceData, PipelineNodeType.SOURCE>;
export type PipelineNodeSink = Node<PipelineNodeSinkData, PipelineNodeType.SINK>;
export type PipelineNode = PipelineNodeSource | PipelineNodeSink | BuiltInNode;

export type PipelineEdge = Edge;

export enum PipelineCanvasEditMode {
  ADD_NODE = "ADD_NODE",
  ADD_EDGE = "ADD_EDGE",
  GRAB = "GRAB",
}

export interface PipelineCanvasState {
  nodes: PipelineNode[];
  edges: PipelineEdge[];
  activeMode: PipelineCanvasEditMode | null;
  isActivityOpen: boolean;
  runBindings: RunBinding[];
}

export interface PipelineCanvasContextShape {
  state: PipelineCanvasState;
  dispatch: React.Dispatch<PipelineCanvasAction>;
}
