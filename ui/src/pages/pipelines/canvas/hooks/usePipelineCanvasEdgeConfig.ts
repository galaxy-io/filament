import { create } from "@bufbuild/protobuf";

import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import { ResourceCursorConfigSchema } from "@/gen/ingestion/v1/pipelines_pb";

import { usePipelineCanvasActions } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import type { PipelineCanvasEdgeModeOptions } from "@/pages/pipelines/canvas/utils";
import {
  getCompatibleWriteModes,
  getReadModeSelectOptions,
  getWriteModeSelectOptions,
} from "@/pages/pipelines/components/resource/utils";

interface PipelineCanvasEdgeConfigOptions extends PipelineCanvasEdgeModeOptions {
  resource: Resource["name"];
  defaultCursor: ResourceColumn["name"];
  hasReadLevers: boolean;
}

export const usePipelineCanvasEdgeConfig = (
  edge: CanvasEdge,
  {
    readModeOptions,
    writeModeOptions,
    effectiveReadMode,
    effectiveWriteMode,
    resource,
    defaultCursor,
    hasReadLevers,
  }: PipelineCanvasEdgeConfigOptions,
) => {
  const { setEdgeConfig, setRouteWriteMode } = usePipelineCanvasActions();

  const configuredReadMode = edge.data?.readMode ?? ReadMode.UNSPECIFIED;
  const configuredWriteMode = edge.data?.writeMode ?? WriteMode.UNSPECIFIED;
  const readMode =
    configuredReadMode === ReadMode.UNSPECIFIED ? effectiveReadMode : configuredReadMode;
  const writeMode =
    configuredWriteMode === WriteMode.UNSPECIFIED ? effectiveWriteMode : configuredWriteMode;
  const cursors = edge.data?.cursors ?? [];
  const cursorConfig = cursors.find((candidate) => candidate.resource === resource);
  const cursor = cursorConfig?.field;

  const createDefaultCursors = () =>
    defaultCursor === ""
      ? []
      : [
          create(ResourceCursorConfigSchema, {
            resource,
            field: defaultCursor,
            lookbackSeconds: 0n,
          }),
        ];

  const compatibleWriteModes = getCompatibleWriteModes(
    writeModeOptions,
    readMode === ReadMode.INCREMENTAL ? [ReadMode.INCREMENTAL] : [],
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
  };

  const handleWriteModeChange = (mode: WriteMode) =>
    hasReadLevers
      ? setEdgeConfig(edge.id, {
          readMode,
          writeMode: mode,
          cursors,
          transform: edge.data?.transform,
        })
      : setRouteWriteMode(edge.source, edge.target, mode);

  const handleCursorChange = (field: ResourceColumn["name"]) =>
    setEdgeConfig(edge.id, {
      readMode,
      writeMode,
      transform: edge.data?.transform,
      cursors: [
        create(ResourceCursorConfigSchema, {
          resource,
          field,
          lookbackSeconds: cursorConfig?.lookbackSeconds ?? 0n,
        }),
      ],
    });

  const readModeSelectOptions = getReadModeSelectOptions(readModeOptions, readMode);
  const writeModeSelectOptions = getWriteModeSelectOptions(compatibleWriteModes, writeMode);

  return {
    configuredReadMode,
    configuredWriteMode,
    readMode,
    writeMode,
    cursors,
    cursor,
    readModeSelectOptions,
    writeModeSelectOptions,
    handleReadModeChange,
    handleWriteModeChange,
    handleCursorChange,
  };
};
