import type { Edge, Node } from "@xyflow/react";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { PipelineEdge, PipelineNode } from "@/gen/ingestion/v1/pipelines_pb";
import type { Resource } from "@/gen/ingestion/v1/providers_pb";

export enum PipelineCanvasNodeType {
  SOURCE = "SOURCE",
  SINK = "SINK",
  PLACEHOLDER = "PLACEHOLDER",
}

export interface PipelineCanvasNodeTableInfo {
  name: Resource["name"];
  isConnected: boolean;
}

export type PipelineCanvasConnectionNodeData = {
  connectionId: PipelineNode["connectionId"];
  config?: NonNullable<PipelineNode["config"]>;
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

export type PipelineCanvasEdgeData = Pick<PipelineEdge, "readMode" | "writeMode" | "cursors">;

export type CanvasEdge = Edge<PipelineCanvasEdgeData>;

export const isConnectionNode = (
  node: CanvasNode,
): node is PipelineCanvasSourceNode | PipelineCanvasSinkNode =>
  node.type === PipelineCanvasNodeType.SOURCE || node.type === PipelineCanvasNodeType.SINK;
