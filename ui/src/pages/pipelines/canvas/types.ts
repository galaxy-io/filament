import type { JsonValue } from "@bufbuild/protobuf";
import type { BuiltInNode, Edge, Node } from "@xyflow/react";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

export enum PipelineNodeType {
  SOURCE = "SOURCE",
  SINK = "SINK",
  PLACEHOLDER = "PLACEHOLDER",
}

export interface PipelineSourceNodeTableInfo {
  name: string;
  isConnected: boolean;
}

export type PipelineConnectionNodeData = {
  label: string;
  connector: string;
  connectionId: string;
  config?: Record<string, JsonValue>;
};

export type PipelinePlaceholderNodeData = {
  kind: ConnectorKind;
};

export type PipelineSourceNode = Node<PipelineConnectionNodeData, PipelineNodeType.SOURCE>;
export type PipelineSinkNode = Node<PipelineConnectionNodeData, PipelineNodeType.SINK>;
export type PipelinePlaceholderNode = Node<
  PipelinePlaceholderNodeData,
  PipelineNodeType.PLACEHOLDER
>;
export type CanvasNode =
  | PipelineSourceNode
  | PipelineSinkNode
  | PipelinePlaceholderNode
  | BuiltInNode;

export type CanvasEdge = Edge;
