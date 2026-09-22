import { useState } from "react";

import { create } from "@bufbuild/protobuf";
import { FlowArrowIcon } from "@phosphor-icons/react";

import Button from "@galaxy-io/dls/buttons/Button";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { ConnectorKind, ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import { ResourceCursorConfigSchema } from "@/gen/ingestion/v1/pipelines_pb";

import ConnectionDrawerKeyValueRow from "@/pages/connectors/components/drawer/ConnectionDrawerKeyValueRow";
import ConnectionDrawerList from "@/pages/connectors/components/drawer/ConnectionDrawerList";
import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { usePipelineCanvasPanelResourceOptions } from "@/pages/pipelines/canvas/panel/hooks/usePipelineCanvasPanelResourceOptions";
import PipelineCanvasPanelResourceCursorField from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceCursorField";
import PipelineCanvasPanelResourceEndpoint from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceEndpoint";
import PipelineCanvasPanelBody from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelBody";
import PipelineCanvasPanelHeader from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelHeader";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { getCanvasEdgeResourceLabel } from "@/pages/pipelines/canvas/utils";
import {
  READ_MODE_TO_LABEL_MAP,
  WRITE_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";
import StreamResourcesTable, {
  type StreamResourceRow,
} from "@/pages/pipelines/components/StreamResourcesTable";
import { streamResourceLabel } from "@/pages/pipelines/streaming";

interface PipelineCanvasPanelResourceDetailProps {
  edge: CanvasEdge;
}

const PipelineCanvasPanelResourceDetail = ({ edge }: PipelineCanvasPanelResourceDetailProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const { edges } = usePipelineCanvasState();
  const { clearSelection, setShowPanel } = usePipelineCanvasSelection();
  const { setEdgeConfig, setRouteWriteMode, applyEdgeChanges } = usePipelineCanvasActions();

  const resource = getCanvasEdgeResource(edge);

  const {
    isCdc,
    isContinuous,
    discoveredResources,
    discoverError,
    isLoading,
    coveredResources,
    readModeOptions,
    writeModeOptions,
    effectiveReadMode,
    effectiveWriteMode,
    cursorOptionsByResource,
    recommendedCursorByResource,
  } = usePipelineCanvasPanelResourceOptions(edge);

  const { label: resourceLabel, isNamedResource } = getCanvasEdgeResourceLabel(
    resource,
    coveredResources.length,
  );

  const routeEdges = edges.filter(
    (candidate) => candidate.source === edge.source && candidate.target === edge.target,
  );
  const [resourceDraft, setResourceDraft] = useState<StreamResourceRow[] | null>(null);
  const initialRows: StreamResourceRow[] = routeEdges
    .filter((candidate) => getCanvasEdgeResource(candidate))
    .map((candidate) => ({
      id: candidate.id,
      subject: getCanvasEdgeResource(candidate),
      label:
        candidate.data?.destinationResource ||
        streamResourceLabel(getCanvasEdgeResource(candidate)),
      selected: true,
    }));
  for (const discovered of discoveredResources) {
    if (discovered.isSelectable && !initialRows.some((row) => row.subject === discovered.name))
      initialRows.push({
        id: `discovered:${discovered.name}`,
        label: streamResourceLabel(discovered.name),
        subject: discovered.name,
        selected: false,
      });
  }
  const resourceRows = resourceDraft ?? initialRows;
  const selectedRows = resourceRows.filter((row) => row.selected);
  const invalidResources =
    !selectedRows.length ||
    selectedRows.some((row) => !row.label.trim() || !row.subject.trim()) ||
    new Set(selectedRows.map((row) => row.label.trim())).size !== selectedRows.length ||
    new Set(selectedRows.map((row) => row.subject.trim())).size !== selectedRows.length;
  const applyResources = () => {
    if (invalidResources) return;
    applyEdgeChanges([
      ...routeEdges.map((candidate) => ({ type: "remove" as const, id: candidate.id })),
      ...selectedRows.map((row) => ({
        type: "add" as const,
        item: {
          ...edge,
          id: `${edge.source}|${row.subject.trim()}|${edge.target}`,
          sourceHandle: row.subject.trim(),
          data: {
            destinationResource: row.label.trim(),
            readMode: ReadMode.UNSPECIFIED,
            writeMode: edge.data?.writeMode || effectiveWriteMode || WriteMode.APPEND,
            cursors: [],
          },
        },
      })),
    ]);
    clearSelection();
  };

  const configuredReadMode = edge.data?.readMode ?? ReadMode.UNSPECIFIED;
  const configuredWriteMode = edge.data?.writeMode ?? WriteMode.UNSPECIFIED;
  const readMode =
    configuredReadMode === ReadMode.UNSPECIFIED ? effectiveReadMode : configuredReadMode;
  const writeMode =
    configuredWriteMode === WriteMode.UNSPECIFIED ? effectiveWriteMode : configuredWriteMode;
  const cursors = edge.data?.cursors ?? [];

  const buildRecommendedCursors = () =>
    coveredResources
      .filter((resourceName) => (recommendedCursorByResource[resourceName] ?? "") !== "")
      .map((resourceName) =>
        create(ResourceCursorConfigSchema, {
          resource: resourceName,
          field: recommendedCursorByResource[resourceName],
          lookbackSeconds: 0n,
        }),
      );

  const routeHasIncremental = edges.some(
    (candidate) =>
      candidate.source === edge.source &&
      candidate.target === edge.target &&
      (candidate.id === edge.id ? readMode : candidate.data?.readMode) === ReadMode.INCREMENTAL,
  );
  const compatibleWriteModes = writeModeOptions.filter(
    (mode) => !routeHasIncremental || mode !== WriteMode.REPLACE,
  );

  const handleReadModeChange = (mode: ReadMode) => {
    const nextWriteMode =
      mode === ReadMode.INCREMENTAL && writeMode === WriteMode.REPLACE
        ? (writeModeOptions.find((candidate) => candidate !== WriteMode.REPLACE) ?? writeMode)
        : writeMode;
    setEdgeConfig(edge.id, {
      readMode: mode,
      writeMode: nextWriteMode,
      cursors: mode === ReadMode.INCREMENTAL ? buildRecommendedCursors() : [],
    });
    if (nextWriteMode !== writeMode) setRouteWriteMode(edge.source, edge.target, nextWriteMode);
  };

  const handleWriteModeChange = (mode: WriteMode) =>
    setRouteWriteMode(edge.source, edge.target, mode);

  const handleCursorChange = (resourceName: Resource["name"], field: ResourceColumn["name"]) =>
    setEdgeConfig(edge.id, {
      readMode,
      writeMode,
      cursors: [
        ...cursors.filter((cursor) => cursor.resource !== resourceName),
        create(ResourceCursorConfigSchema, {
          resource: resourceName,
          field,
          lookbackSeconds: 0n,
        }),
      ],
    });

  const cursorsByResource = new Map(cursors.map((cursor) => [cursor.resource, cursor.field]));

  const readModeSelectOptions: SelectInputOption[] = readModeOptions.map((mode) => ({
    id: String(mode),
    label: READ_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));
  const writeModeSelectOptions: SelectInputOption[] = compatibleWriteModes.map((mode) => ({
    id: String(mode),
    label: WRITE_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));

  return (
    <>
      <PipelineCanvasPanelHeader
        title={resourceLabel}
        icon={FlowArrowIcon}
        onBack={clearSelection}
        onClose={() => setShowPanel(false)}
        onDelete={
          isReadOnly
            ? undefined
            : () => {
                applyEdgeChanges([{ id: edge.id, type: "remove" }]);
                clearSelection();
              }
        }
      />
      <PipelineCanvasPanelBody>
        <ConnectionDrawerList>
          <ConnectionDrawerKeyValueRow
            label="Source"
            value={
              <PipelineCanvasPanelResourceEndpoint
                nodeId={edge.source}
                kind={ConnectorKind.SOURCE}
              />
            }
          />
          <ConnectionDrawerKeyValueRow
            label="Resource"
            value={
              <Text size={TextSize.BODY_SM} isMonospace={isNamedResource}>
                {resourceLabel}
              </Text>
            }
          />
          <ConnectionDrawerKeyValueRow
            label="Sink"
            value={
              <PipelineCanvasPanelResourceEndpoint nodeId={edge.target} kind={ConnectorKind.SINK} />
            }
          />
        </ConnectionDrawerList>
        <PipelineCanvasPanelSection
          header="Configuration"
          isEmpty={false}
          emptyHeader="No configuration"
          emptyMessage="No configuration options are available for this resource."
          padding="12px"
        >
          <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
            {isContinuous && (
              <>
                {discoverError && (
                  <Text>Discovery is unavailable. Add subjects or topics manually.</Text>
                )}
                <StreamResourcesTable
                  rows={resourceRows}
                  onChange={(row) =>
                    setResourceDraft(
                      resourceRows.map((candidate) => (candidate.id === row.id ? row : candidate)),
                    )
                  }
                  onSelection={(selection) =>
                    setResourceDraft(
                      resourceRows.map((row) => ({ ...row, selected: !!selection[row.id] })),
                    )
                  }
                  onAdd={() =>
                    setResourceDraft([
                      ...resourceRows,
                      { id: crypto.randomUUID(), label: "", subject: "", selected: true },
                    ])
                  }
                  isDisabled={isReadOnly}
                />
                <Text>Changes take effect after stopping the stream and starting a new run.</Text>
                {resourceDraft && invalidResources && (
                  <Text>
                    Select at least one resource with a unique, non-empty label and subject/topic.
                  </Text>
                )}
                <Button
                  label="Apply resources"
                  onClick={applyResources}
                  isDisabled={isReadOnly || invalidResources || resourceDraft === null}
                />
              </>
            )}
            {!isCdc && !isContinuous && (
              <SelectInput
                label="Read mode"
                options={readModeSelectOptions}
                value={readModeSelectOptions.find((option) => option.value === readMode) ?? null}
                onChange={(option) => handleReadModeChange(option.value as ReadMode)}
                variant={InputVariant.TERTIARY}
                placeholder="Select a read mode..."
                size={InputSize.LARGE}
                isDisabled={isReadOnly || isLoading}
                fillWidth
              />
            )}
            <SelectInput
              label="Write mode"
              options={writeModeSelectOptions}
              value={writeModeSelectOptions.find((option) => option.value === writeMode) ?? null}
              onChange={(option) => handleWriteModeChange(option.value as WriteMode)}
              variant={InputVariant.TERTIARY}
              placeholder="Select a write mode..."
              size={InputSize.LARGE}
              isDisabled={isReadOnly || isLoading}
              fillWidth
            />
            {!isContinuous &&
              readMode === ReadMode.INCREMENTAL &&
              coveredResources.map((resourceName) => (
                <PipelineCanvasPanelResourceCursorField
                  key={resourceName}
                  value={cursorsByResource.get(resourceName) ?? ""}
                  options={cursorOptionsByResource[resourceName] ?? []}
                  isDisabled={isReadOnly || isLoading}
                  onChange={(field) => handleCursorChange(resourceName, field)}
                />
              ))}
          </FlexWrapper>
        </PipelineCanvasPanelSection>
      </PipelineCanvasPanelBody>
    </>
  );
};

export default PipelineCanvasPanelResourceDetail;
