import type { BuiltInNode, Edge, Node } from "@xyflow/react";

import type { PipelineCanvasAction } from "@/pages/pipelines/canvas/actions";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { RunBinding } from "@/gen/ingestion/v1/runs_pb";

export enum PipelineNodeType {
  SOURCE = "SOURCE",
  SINK = "SINK",
  PLACEHOLDER = "PLACEHOLDER",
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

export type PipelineNodePlaceholderData = {
  kind: ConnectorKind;
};

export type PipelineNodeSource = Node<PipelineNodeSourceData, PipelineNodeType.SOURCE>;
export type PipelineNodeSink = Node<PipelineNodeSinkData, PipelineNodeType.SINK>;
export type PipelineNodePlaceholder = Node<
  PipelineNodePlaceholderData,
  PipelineNodeType.PLACEHOLDER
>;
export type PipelineNode =
  | PipelineNodeSource
  | PipelineNodeSink
  | PipelineNodePlaceholder
  | BuiltInNode;

export type PipelineEdge = Edge;

export enum PipelineCanvasEditMode {
  ADD_NODE = "ADD_NODE",
}

export enum PipelineCanvasInteractionMode {
  GRAB = "GRAB",
  SELECT = "SELECT",
}

export interface PipelineCanvasState {
  nodes: PipelineNode[];
  edges: PipelineEdge[];
  isReadOnly: boolean;
  activeMode: PipelineCanvasEditMode | null;
  interactionMode: PipelineCanvasInteractionMode;
  isActivityOpen: boolean;
  runBindings: RunBinding[];
}

export interface PipelineCanvasContextShape {
  state: PipelineCanvasState;
  dispatch: React.Dispatch<PipelineCanvasAction>;
}
