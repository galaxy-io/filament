import type { Node, Edge, BuiltInNode } from "@xyflow/react";

// Node type identifiers
export enum PipelineNodeType {
  SOURCE = "source",
  SINK = "sink",
}

// Connector types (data sources/destinations)
export enum ConnectorType {
  POSTGRES = "postgres",
  S3 = "s3",
  STDOUT = "stdout",
}

// Table info for source nodes
export interface PipelineNodeSourceTableInfo {
  name: string;
  rowCount: string;
  isConnected: boolean;
}

// Source node specific data
export type PipelineNodeSourceData = {
  label: string;
  connectorType: ConnectorType;
  tables?: PipelineNodeSourceTableInfo[];
};

// Sink node specific data
export type PipelineNodeSinkData = {
  label: string;
  connectorType: ConnectorType;
};

// Typed nodes
export type PipelineNodeSource = Node<PipelineNodeSourceData, PipelineNodeType.SOURCE>;
export type PipelineNodeSink = Node<PipelineNodeSinkData, PipelineNodeType.SINK>;
export type PipelineNode = PipelineNodeSource | PipelineNodeSink | BuiltInNode;

// Typed edges
export type PipelineEdge = Edge;

// Edit widget modes
export enum PipelineCanvasEditMode {
  ADD = "add",
  EDIT = "edit",
  ACTIVITY = "activity",
}
