import { create } from "@bufbuild/protobuf";

import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import { ResourceCursorConfigSchema } from "@/gen/ingestion/v1/pipelines_pb";

import {
  usePipelineCanvasActions,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import {
  hasSiblingIncrementalRead,
  type PipelineCanvasEdgeModeOptions,
} from "@/pages/pipelines/canvas/utils";
import {
  getCompatibleWriteModes,
  getReadModeSelectOptions,
  getWriteModeSelectOptions,
} from "@/pages/pipelines/components/resource/utils";

interface PipelineCanvasEdgeConfigOptions extends PipelineCanvasEdgeModeOptions {
  coveredResources: Resource["name"][];
  defaultCursorByResource: Record<Resource["name"], ResourceColumn["name"]>;
}

export const usePipelineCanvasEdgeConfig = (
  edge: CanvasEdge,
  {
    readModeOptions,
    writeModeOptions,
    effectiveReadMode,
    effectiveWriteMode,
    coveredResources,
    defaultCursorByResource,
  }: PipelineCanvasEdgeConfigOptions,
) => {
  const { edges } = usePipelineCanvasState();
  const { setEdgeConfig, setRouteWriteMode } = usePipelineCanvasActions();

  const configuredReadMode = edge.data?.readMode ?? ReadMode.UNSPECIFIED;
  const configuredWriteMode = edge.data?.writeMode ?? WriteMode.UNSPECIFIED;
  const readMode =
    configuredReadMode === ReadMode.UNSPECIFIED ? effectiveReadMode : configuredReadMode;
  const writeMode =
    configuredWriteMode === WriteMode.UNSPECIFIED ? effectiveWriteMode : configuredWriteMode;
  const cursors = edge.data?.cursors ?? [];
  const cursorsByResource = new Map(cursors.map((cursor) => [cursor.resource, cursor.field]));

  const createDefaultCursors = () =>
    coveredResources
      .filter((resourceName) => (defaultCursorByResource[resourceName] ?? "") !== "")
      .map((resourceName) =>
        create(ResourceCursorConfigSchema, {
          resource: resourceName,
          field: defaultCursorByResource[resourceName],
          lookbackSeconds: 0n,
        }),
      );

  const routeHasIncremental =
    readMode === ReadMode.INCREMENTAL || hasSiblingIncrementalRead(edges, edge, edge.id);
  const compatibleWriteModes = getCompatibleWriteModes(
    writeModeOptions,
    routeHasIncremental ? [ReadMode.INCREMENTAL] : [],
  );

  const handleReadModeChange = (mode: ReadMode) => {
    const nextWriteMode =
      mode === ReadMode.INCREMENTAL && writeMode === WriteMode.REPLACE
        ? writeModeOptions.includes(WriteMode.UPSERT)
          ? WriteMode.UPSERT
          : (writeModeOptions.find((candidate) => candidate !== WriteMode.REPLACE) ?? writeMode)
        : writeMode;
    setEdgeConfig(edge.id, {
      readMode: mode,
      writeMode: nextWriteMode,
      cursors: mode === ReadMode.INCREMENTAL ? createDefaultCursors() : [],
      transform: edge.data?.transform,
    });
    if (nextWriteMode !== writeMode) setRouteWriteMode(edge.source, edge.target, nextWriteMode);
  };

  const handleWriteModeChange = (mode: WriteMode) =>
    setRouteWriteMode(edge.source, edge.target, mode);

  const handleCursorChange = (resourceName: Resource["name"], field: ResourceColumn["name"]) =>
    setEdgeConfig(edge.id, {
      readMode,
      writeMode,
      transform: edge.data?.transform,
      cursors: [
        ...cursors.filter((cursor) => cursor.resource !== resourceName),
        create(ResourceCursorConfigSchema, {
          resource: resourceName,
          field,
          lookbackSeconds: 0n,
        }),
      ],
    });

  const readModeSelectOptions = getReadModeSelectOptions(readModeOptions);
  const writeModeSelectOptions = getWriteModeSelectOptions(compatibleWriteModes);

  return {
    configuredReadMode,
    configuredWriteMode,
    readMode,
    writeMode,
    cursors,
    cursorsByResource,
    readModeSelectOptions,
    writeModeSelectOptions,
    handleReadModeChange,
    handleWriteModeChange,
    handleCursorChange,
  };
};
