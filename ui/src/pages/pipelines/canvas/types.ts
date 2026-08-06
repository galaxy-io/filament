import type { JsonValue } from "@bufbuild/protobuf";
import type { Edge, Node } from "@xyflow/react";

import type { ConnectorKind, IngestionType } from "@/gen/ingestion/v1/common_pb";
import type { ResourceCursorConfig } from "@/gen/ingestion/v1/pipelines_pb";

export enum PipelineCanvasNodeType {
  SOURCE = "SOURCE",
  SINK = "SINK",
  PLACEHOLDER = "PLACEHOLDER",
}

export interface PipelineCanvasNodeResourceInfo {
  name: string;
  isConnected: boolean;
}

export type PipelineCanvasConnectionNodeData = {
  label: string;
  connector: string;
  connectionId: string;
  config?: Record<string, JsonValue>;
};

export type PipelineCanvasPlaceholderNodeData = {
  kind: ConnectorKind;
};

export type PipelineCanvasSourceNode = Node<
  PipelineCanvasConnectionNodeData,
  PipelineCanvasNodeType.SOURCE
>;
export type PipelineCanvasSinkNode = Node<
  PipelineCanvasConnectionNodeData,
  PipelineCanvasNodeType.SINK
>;
export type PipelineCanvasPlaceholderNode = Node<
  PipelineCanvasPlaceholderNodeData,
  PipelineCanvasNodeType.PLACEHOLDER
>;
export type CanvasNode =
  | PipelineCanvasSourceNode
  | PipelineCanvasSinkNode
  | PipelineCanvasPlaceholderNode;

export type PipelineCanvasEdgeData = {
  ingestionType: IngestionType;
  selector: string;
  cursors: ResourceCursorConfig[];
};

export type PipelineCanvasWireEdge = PipelineCanvasEdgeData & {
  fromNode: string;
  resource: string;
  toNode: string;
};

export type CanvasEdge = Edge<PipelineCanvasEdgeData>;

export const isConnectionNode = (
  node: CanvasNode,
): node is PipelineCanvasSourceNode | PipelineCanvasSinkNode =>
  node.type === PipelineCanvasNodeType.SOURCE || node.type === PipelineCanvasNodeType.SINK;
