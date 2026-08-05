import { create } from "@bufbuild/protobuf";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import {
  type CreatePipelineRequest,
  CreatePipelineRequestSchema,
} from "@/gen/ingestion/v1/pipelines_pb";

import {
  PIPELINE_CANVAS_EDGE_TYPE,
  PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
  PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
} from "@/pages/pipelines/canvas/constants";
import { getNextNodePosition } from "@/pages/pipelines/canvas/graph/layout";
import { createNodeFromConnection } from "@/pages/pipelines/canvas/graph/rules";
import {
  type CanvasEdge,
  type CanvasNode,
  PipelineCanvasNodeType,
} from "@/pages/pipelines/canvas/types";
import type { CreatePipelineModalState } from "@/pages/pipelines/components/create/types";
import { mapPipelineScheduleStateToCron } from "@/pages/pipelines/settings/utils";

export const getDefaultPipelineName = (source: Connection | null, sinks: Connection[]): string => {
  const sinkNames = sinks.map((sink) => sink.name).join(", ");
  if (source && sinks.length) return `${source.name} -> ${sinkNames}`;
  if (source) return source.name;
  return sinkNames;
};

export const mapCreatePipelineSelectionToCanvasState = (
  source: Connection | null,
  sinks: Connection[],
): { nodes: CanvasNode[]; edges: CanvasEdge[] } => {
  const nodes: CanvasNode[] = [];
  for (const connection of [...(source ? [source] : []), ...sinks]) {
    nodes.push(createNodeFromConnection(connection, getNextNodePosition(connection.kind, nodes)));
  }

  const sourceNode = nodes.find((node) => node.type === PipelineCanvasNodeType.SOURCE);
  const edges: CanvasEdge[] = sourceNode
    ? nodes
        .filter((node) => node.type === PipelineCanvasNodeType.SINK)
        .map((sinkNode) => ({
          id: `${sourceNode.id}||${sinkNode.id}`,
          type: PIPELINE_CANVAS_EDGE_TYPE,
          source: sourceNode.id,
          target: sinkNode.id,
          sourceHandle: PIPELINE_CANVAS_NODE_SOURCE_HANDLE_ID,
          targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
        }))
    : [];

  return { nodes, edges };
};

export const mapCreatePipelineStateToRequest = (
  state: CreatePipelineModalState,
  name: string,
): CreatePipelineRequest =>
  create(CreatePipelineRequestSchema, {
    name: name.trim(),
    description: state.description.trim(),
    schedule: state.schedule.isEnabled
      ? {
          cron: mapPipelineScheduleStateToCron(state.schedule),
          timezone: state.schedule.timezone,
          enabled: true,
        }
      : undefined,
  });
