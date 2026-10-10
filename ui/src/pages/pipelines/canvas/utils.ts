import type { JsonValue } from "@bufbuild/protobuf";
import type { FitViewOptions } from "@xyflow/react";

import type { Theme } from "@galaxy-io/dls/theme/tokens/types";

import type { EdgeValidation } from "@/gen/ingestion/v1/capabilities_pb";
import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { PipelineEdge } from "@/gen/ingestion/v1/pipelines_pb";

import { isJsonObject } from "@/components/fields/utils";
import type { PipelineFlowEndpoints } from "@/components/pipelines/utils";

import {
  PIPELINE_CANVAS_EDGE_Z_INDEX,
  PIPELINE_CANVAS_FIT_INSET_LEFT,
  PIPELINE_CANVAS_FIT_INSET_Y,
  PIPELINE_CANVAS_FIT_MAX_ZOOM,
} from "@/pages/pipelines/canvas/constants";
import type { PipelineCanvasEdgeResources } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasEdgeResources";
import {
  PIPELINE_CANVAS_PANEL_COLLAPSED_WIDTH,
  PIPELINE_CANVAS_PANEL_INSET,
  PIPELINE_CANVAS_PANEL_WIDTH,
} from "@/pages/pipelines/canvas/panel/constants";
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
} from "@/pages/pipelines/components/resource/utils";

const intersectModes = (sets: ReadMode[][]): ReadMode[] => {
  if (!sets.length) return [];
  return sets
    .slice(1)
    .reduce((common, modes) => common.filter((mode) => modes.includes(mode)), sets[0] ?? []);
};

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
      ? intersectModes(verdict.resources.map((resource) => resource.supportedReadModes))
      : [],
  writeModeOptions: verdict?.supportedWriteModes ?? [],
  effectiveReadMode: verdict?.effectiveReadMode ?? ReadMode.UNSPECIFIED,
  effectiveWriteMode: verdict?.effectiveWriteMode ?? WriteMode.UNSPECIFIED,
});

type PipelineCanvasEdgeStatusResources = Pick<
  PipelineCanvasEdgeResources,
  | "hasReadLevers"
  | "coveredResources"
  | "columnsByResource"
  | "cursorOptionsByResource"
  | "managedIncrementalResources"
  | "primaryKeyByResource"
>;

interface PipelineCanvasEdgeStatusConfig {
  verdict: EdgeValidation | undefined;
  readMode: ReadMode;
  writeMode: WriteMode;
  cursorsByResource: Map<Resource["name"], ResourceColumn["name"]>;
}

export const getEdgeResourceStatuses = (
  {
    hasReadLevers,
    coveredResources,
    columnsByResource,
    cursorOptionsByResource,
    managedIncrementalResources,
    primaryKeyByResource,
  }: PipelineCanvasEdgeStatusResources,
  { verdict, readMode, writeMode, cursorsByResource }: PipelineCanvasEdgeStatusConfig,
): PipelineResourceStatus[] => {
  if (!hasReadLevers || verdict === undefined) return [];
  const { readModeOptions } = getEdgeModeOptions(verdict, hasReadLevers);
  const supportedReadModesByResource = new Map(
    verdict.resources.map((resource) => [resource.resource, resource.supportedReadModes]),
  );
  return coveredResources.flatMap((resource) => {
    const columns = columnsByResource.get(resource);
    const status = getPipelineResourceStatus({
      resource,
      readMode,
      readModeOptions: supportedReadModesByResource.get(resource) ?? readModeOptions,
      cursorField: cursorsByResource.get(resource) ?? getRecommendedCursor(columns ?? []),
      cursorOptions: cursorOptionsByResource[resource] ?? [],
      isCursorKnown: columns !== undefined,
      managedIncremental: managedIncrementalResources.has(resource),
      hasPrimaryKey: (primaryKeyByResource[resource]?.length ?? 0) > 0,
      needsPrimaryKey: writeMode === WriteMode.UPSERT,
    });
    return status ? [status] : [];
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
  if (resource !== "") return getTransformEntryStepCount(resources[resource]);
  return Object.values(resources).reduce<number>(
    (total, entry) => total + getTransformEntryStepCount(entry),
    0,
  );
};

export const getPipelineCanvasFitPadding = (
  showPanel: boolean,
): NonNullable<FitViewOptions["padding"]> => ({
  top: `${PIPELINE_CANVAS_FIT_INSET_Y}px`,
  bottom: `${PIPELINE_CANVAS_FIT_INSET_Y}px`,
  left: `${PIPELINE_CANVAS_FIT_INSET_LEFT}px`,
  right: `${
    PIPELINE_CANVAS_PANEL_INSET * 2 +
    (showPanel ? PIPELINE_CANVAS_PANEL_WIDTH : PIPELINE_CANVAS_PANEL_COLLAPSED_WIDTH)
  }px`,
});

export const getPipelineCanvasFitViewOptions = (showPanel: boolean): FitViewOptions => ({
  padding: getPipelineCanvasFitPadding(showPanel),
  maxZoom: PIPELINE_CANVAS_FIT_MAX_ZOOM,
});

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

export const getCanvasEdgeResourceLabel = (
  resource: PipelineEdge["resource"],
  coveredCount: number,
): { label: string; isNamedResource: boolean } => {
  if (resource) return { label: resource, isNamedResource: true };
  return {
    label: coveredCount ? `${coveredCount} resources` : "All resources",
    isNamedResource: false,
  };
};

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
