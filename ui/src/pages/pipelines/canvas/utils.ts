import type { JsonValue } from "@bufbuild/protobuf";

import type { Theme } from "@galaxy-io/dls/theme/tokens/types";

import type { EdgeValidation } from "@/gen/ingestion/v1/capabilities_pb";
import { ConnectorKind, ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { PipelineEdge } from "@/gen/ingestion/v1/pipelines_pb";

import { isJsonObject } from "@/components/fields/utils";
import type { PipelineFlowEndpoints } from "@/components/pipelines/utils";

import { PIPELINE_CANVAS_EDGE_Z_INDEX } from "@/pages/pipelines/canvas/constants";
import type { PipelineCanvasEdgeResources } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasEdgeResources";
import type {
  CanvasEdge,
  CanvasNode,
  PipelineCanvasEdgeTransform,
} from "@/pages/pipelines/canvas/types";
import {
  PipelineCanvasNodeType,
  type PipelineCanvasSinkNode,
  type PipelineCanvasSourceNode,
} from "@/pages/pipelines/canvas/types";
import type { PipelineResourceStatus } from "@/pages/pipelines/components/resource/types";
import {
  getPipelineResourceStatus,
  getRecommendedCursor,
  getSharedModes,
} from "@/pages/pipelines/components/resource/utils";

export interface PipelineCanvasEdgeModeOptions {
  readModeOptions: ReadMode[];
  writeModeOptions: WriteMode[];
  effectiveReadMode: ReadMode;
  effectiveWriteMode: WriteMode;
}

export const getEdgeModeOptions = (
  verdict: EdgeValidation | undefined,
  hasReadLevers: boolean,
): PipelineCanvasEdgeModeOptions => ({
  readModeOptions:
    verdict && hasReadLevers
      ? getSharedModes(verdict.resources.map((resource) => resource.supportedReadModes))
      : [],
  writeModeOptions: verdict?.supportedWriteModes ?? [],
  effectiveReadMode: verdict?.effectiveReadMode ?? ReadMode.UNSPECIFIED,
  effectiveWriteMode: verdict?.effectiveWriteMode ?? WriteMode.UNSPECIFIED,
});

type PipelineCanvasEdgeStatusResources = Pick<
  PipelineCanvasEdgeResources,
  | "hasReadLevers"
  | "resource"
  | "columns"
  | "cursorOptions"
  | "isManagedIncremental"
  | "hasPrimaryKey"
>;

interface PipelineCanvasEdgeStatusConfig {
  verdict: EdgeValidation | undefined;
  readMode: ReadMode;
  writeMode: WriteMode;
  cursor: ResourceColumn["name"] | undefined;
}

export const getEdgeResourceStatus = (
  {
    hasReadLevers,
    resource,
    columns,
    cursorOptions,
    isManagedIncremental,
    hasPrimaryKey,
  }: PipelineCanvasEdgeStatusResources,
  { verdict, readMode, writeMode, cursor }: PipelineCanvasEdgeStatusConfig,
): PipelineResourceStatus | undefined => {
  if (!hasReadLevers || verdict === undefined || resource === "") return undefined;
  return getPipelineResourceStatus({
    resource,
    readMode,
    readModeOptions:
      verdict.resources.find((candidate) => candidate.resource === resource)?.supportedReadModes ??
      getEdgeModeOptions(verdict, hasReadLevers).readModeOptions,
    cursorField: cursor ?? getRecommendedCursor(columns ?? []),
    cursorOptions,
    isCursorKnown: columns !== undefined,
    managedIncremental: isManagedIncremental,
    hasPrimaryKey,
    needsPrimaryKey: writeMode === WriteMode.UPSERT,
  });
};

export const hasSiblingIncrementalRead = (
  edges: CanvasEdge[],
  { source, target }: Pick<CanvasEdge, "source" | "target">,
  excludedEdgeId?: CanvasEdge["id"],
): boolean =>
  edges.some(
    (edge) =>
      edge.source === source &&
      edge.target === target &&
      edge.id !== excludedEdgeId &&
      edge.data?.readMode === ReadMode.INCREMENTAL,
  );

const getTransformEntryStepCount = (entry: JsonValue | undefined): number =>
  entry !== undefined && isJsonObject(entry) && Array.isArray(entry.steps) ? entry.steps.length : 0;

export const getTransformStepCount = (
  transform: PipelineCanvasEdgeTransform | undefined,
  resource: PipelineEdge["resource"],
): number => {
  const resources = transform?.resources;
  if (resources === undefined || !isJsonObject(resources)) return 0;
  return getTransformEntryStepCount(resources[resource]);
};

export const mapElementsToSelected = <T extends { id: string; selected?: boolean }>(
  elements: T[],
  selectedId: string | undefined,
): T[] =>
  elements.map((element) => {
    const selected = element.id === selectedId;
    return element.selected === selected ? element : { ...element, selected };
  });

export const getDefaultDestinationResource = (resource: PipelineEdge["resource"]): string =>
  resource.replace(/[^a-zA-Z0-9_]+/g, "_").replace(/^_+|_+$/g, "");

export const mapEdgesToStyledEdges = (
  edges: CanvasEdge[],
  nodes: CanvasNode[],
  theme: Theme,
  isRunning: boolean,
  invalidEdgeIds: Set<CanvasEdge["id"]>,
): CanvasEdge[] => {
  const selectedNodeIds = new Set(nodes.filter((node) => node.selected).map((node) => node.id));

  return edges.map((edge) => {
    const isConnectedToSelected =
      selectedNodeIds.has(edge.source) || selectedNodeIds.has(edge.target);
    const isHighlighted = edge.selected || isConnectedToSelected;
    const isInvalid = invalidEdgeIds.has(edge.id);
    const stroke = isInvalid
      ? theme.color.text.error
      : isRunning || isHighlighted
        ? theme.color.solid.primary.background
        : theme.color.border.primary;

    return {
      ...edge,
      zIndex: edge.selected
        ? PIPELINE_CANVAS_EDGE_Z_INDEX + 2
        : isInvalid || isHighlighted
          ? PIPELINE_CANVAS_EDGE_Z_INDEX + 1
          : PIPELINE_CANVAS_EDGE_Z_INDEX,
      animated: isRunning,
      style: {
        stroke,
        strokeWidth: edge.selected ? 3 : 2,
        ...(isRunning && { strokeDasharray: "5 5" }),
      },
    };
  });
};

export const mapCanvasNodesToFlowEndpoints = (nodes: CanvasNode[]): PipelineFlowEndpoints => ({
  sourceId: nodes.find(
    (node): node is PipelineCanvasSourceNode => node.type === PipelineCanvasNodeType.SOURCE,
  )?.data.connectionId,
  sinkIds: nodes
    .filter((node): node is PipelineCanvasSinkNode => node.type === PipelineCanvasNodeType.SINK)
    .map((node) => node.data.connectionId),
});

export const isConnectionNode = (
  node: CanvasNode,
): node is PipelineCanvasSourceNode | PipelineCanvasSinkNode =>
  node.type === PipelineCanvasNodeType.SOURCE || node.type === PipelineCanvasNodeType.SINK;

export const isPipelineCanvasConnectionDisabled = (
  connection: Connection,
  isSourceDisabled: boolean,
) => isSourceDisabled && connection.kind === ConnectorKind.SOURCE;
