import type { Connection as CanvasConnection } from "@xyflow/react";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import {
  PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
  PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
  PIPELINE_CANVAS_NODE_TYPE_TO_COUNTERPART_TYPE_MAP,
} from "@/pages/pipelines/canvas/constants";
import {
  type CanvasNode,
  isConnectionNode,
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";

export const createNodeFromConnection = (
  connection: Connection,
  position: { x: number; y: number },
): CanvasNode => {
  const data = {
    label: connection.name,
    connector: connection.connector,
    connectionId: connection.id,
  };

  if (connection.kind === ConnectorKind.SOURCE) {
    return { id: crypto.randomUUID(), type: PipelineCanvasNodeType.SOURCE, position, data };
  }

  return { id: crypto.randomUUID(), type: PipelineCanvasNodeType.SINK, position, data };
};

export const canAddSourceNode = (nodes: CanvasNode[]): boolean =>
  !nodes.some((node) => node.type === PipelineCanvasNodeType.SOURCE);

export const getAutoConnections = (node: CanvasNode, nodes: CanvasNode[]): CanvasConnection[] => {
  if (!isConnectionNode(node) || nodes.some((existing) => existing.type === node.type)) {
    return [];
  }

  const isSource = node.type === PipelineCanvasNodeType.SOURCE;
  const counterpartType = PIPELINE_CANVAS_NODE_TYPE_TO_COUNTERPART_TYPE_MAP[node.type];

  return nodes
    .filter((counterpart) => counterpart.type === counterpartType)
    .map((counterpart) => ({
      source: isSource ? node.id : counterpart.id,
      sourceHandle: PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
      target: isSource ? counterpart.id : node.id,
      targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
    }));
};
