import type { EdgeValidation } from "@/gen/ingestion/v1/capabilities_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { PipelineEdge, ResourceCursorConfig } from "@/gen/ingestion/v1/pipelines_pb";

import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";

export interface PipelineCanvasRoute {
  edge: CanvasEdge;
  resource: PipelineEdge["resource"];
  resourceLabel: string;
  isNamedResource: boolean;
  sourceConnection: Connection | undefined;
  sinkConnection: Connection | undefined;
  groupKey: string;
  groupIndex: number;
  groupSize: number;
  hasReadLevers: boolean;
  transformStepCount: number;
  issues: string[];
  isInvalid: boolean;
  verdict: EdgeValidation | undefined;
}

export interface PipelineCanvasRoutesDraftConfig {
  readMode: PipelineEdge["readMode"] | undefined;
  writeMode: PipelineEdge["writeMode"] | undefined;
  cursor: ResourceCursorConfig["field"] | undefined;
}

export interface PipelineCanvasRoutesDraft extends PipelineCanvasRoutesDraftConfig {
  resource: PipelineEdge["resource"] | undefined;
  sinkId: CanvasNode["id"];
  isResourceLocked: boolean;
}

export enum PipelineCanvasRoutesListItemKind {
  ROUTE = "route",
  DRAFT = "draft",
}

export type PipelineCanvasRoutesListItem =
  | { kind: PipelineCanvasRoutesListItemKind.ROUTE; route: PipelineCanvasRoute }
  | { kind: PipelineCanvasRoutesListItemKind.DRAFT };
