import { create } from "@bufbuild/protobuf";
import { FlowArrowIcon } from "@phosphor-icons/react";

import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, {
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

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
import PipelineCanvasPanelHeader from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelHeader";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
  usePipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import {
  getCanvasEdgeResourceLabel,
  getDefaultDestinationResource,
} from "@/pages/pipelines/canvas/utils";
import {
  READ_MODE_TO_LABEL_MAP,
  WRITE_MODE_TO_LABEL_MAP,
} from "@/pages/pipelines/components/create/constants";

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
    isContinuous,
    hasReadLevers,
    isLoading,
    coveredResources,
    readModeOptions,
    writeModeOptions,
    effectiveReadMode,
    effectiveWriteMode,
    cursorOptionsByResource,
    recommendedCursorByResource,
    managedIncrementalResources,
  } = usePipelineCanvasPanelResourceOptions(edge);

  const { label: resourceLabel, isNamedResource } = getCanvasEdgeResourceLabel(
    resource,
    coveredResources.length,
  );

  const configuredReadMode = edge.data?.readMode ?? ReadMode.UNSPECIFIED;
  const configuredWriteMode = edge.data?.writeMode ?? WriteMode.UNSPECIFIED;
  const readMode =
    configuredReadMode === ReadMode.UNSPECIFIED ? effectiveReadMode : configuredReadMode;
  const writeMode =
    configuredWriteMode === WriteMode.UNSPECIFIED ? effectiveWriteMode : configuredWriteMode;
  const cursors = edge.data?.cursors ?? [];
  const destinationResource = edge.data?.destinationResource ?? "";

  const buildRecommendedCursors = () =>
    coveredResources
      .filter(
        (resourceName) =>
          !managedIncrementalResources.has(resourceName) &&
          (recommendedCursorByResource[resourceName] ?? "") !== "",
      )
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
        ? writeModeOptions.includes(WriteMode.UPSERT)
          ? WriteMode.UPSERT
          : (writeModeOptions.find((candidate) => candidate !== WriteMode.REPLACE) ?? writeMode)
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

  const handleDestinationChange = (value: string) =>
    setEdgeConfig(edge.id, { readMode, writeMode, cursors, destinationResource: value.trim() });

  const cursorsByResource = new Map(cursors.map((cursor) => [cursor.resource, cursor.field]));

  const readModeSelectOptions: SelectOption[] = readModeOptions.map((mode) => ({
    id: String(mode),
    label: READ_MODE_TO_LABEL_MAP[mode],
  }));
  const writeModeSelectOptions: SelectOption[] = compatibleWriteModes.map((mode) => ({
    id: String(mode),
    label: WRITE_MODE_TO_LABEL_MAP[mode],
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
      <FlexItem grow={1} minHeight={0}>
        <ScrollArea>
          <Flex direction={FlexDirection.COLUMN} gap={8} padding={12}>
            <ConnectionDrawerList variant={BoxVariant.SECONDARY}>
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
                  <Text
                    size={TextSize.BODY_SM}
                    family={isNamedResource ? FontFamily.MONO : FontFamily.SANS}
                  >
                    {resourceLabel}
                  </Text>
                }
              />
              <ConnectionDrawerKeyValueRow
                label="Sink"
                value={
                  <PipelineCanvasPanelResourceEndpoint
                    nodeId={edge.target}
                    kind={ConnectorKind.SINK}
                  />
                }
              />
            </ConnectionDrawerList>
            <PipelineCanvasPanelSection
              header="Configuration"
              isEmpty={false}
              emptyHeader="No configuration"
              emptyMessage="No configuration options are available for this resource."
              hasInset
            >
              <Flex
                alignItems={AlignItems.START}
                direction={FlexDirection.COLUMN}
                gap={12}
                fillWidth
              >
                {hasReadLevers && (
                  <SelectInput
                    label="Read mode"
                    options={readModeSelectOptions}
                    value={String(readMode)}
                    onChange={(id) => {
                      if (id !== null) handleReadModeChange(Number(id) as ReadMode);
                    }}
                    placeholder="Select a read mode..."
                    isDisabled={isReadOnly || isLoading}
                    variant={SelectInputVariant.TERTIARY}
                    fillWidth
                  />
                )}
                <SelectInput
                  label="Write mode"
                  options={writeModeSelectOptions}
                  value={String(writeMode)}
                  onChange={(id) => {
                    if (id !== null) handleWriteModeChange(Number(id) as WriteMode);
                  }}
                  placeholder="Select a write mode..."
                  isDisabled={isReadOnly || isLoading}
                  variant={SelectInputVariant.TERTIARY}
                  fillWidth
                />
                {isContinuous && isNamedResource && (
                  <TextInput
                    label="Destination"
                    value={destinationResource}
                    onChange={handleDestinationChange}
                    placeholder={getDefaultDestinationResource(resource)}
                    isDisabled={isReadOnly}
                    variant={InputVariant.TERTIARY}
                    fillWidth
                  />
                )}
                {hasReadLevers &&
                  readMode === ReadMode.INCREMENTAL &&
                  coveredResources.map((resourceName) =>
                    managedIncrementalResources.has(resourceName) ? (
                      <Text key={resourceName} size={TextSize.BODY_SM}>
                        {resourceName}: incremental state is managed by the source.
                      </Text>
                    ) : (
                      <PipelineCanvasPanelResourceCursorField
                        key={resourceName}
                        value={cursorsByResource.get(resourceName) ?? ""}
                        options={cursorOptionsByResource[resourceName] ?? []}
                        isDisabled={isReadOnly || isLoading}
                        onChange={(field) => handleCursorChange(resourceName, field)}
                      />
                    ),
                  )}
              </Flex>
            </PipelineCanvasPanelSection>
          </Flex>
        </ScrollArea>
      </FlexItem>
    </>
  );
};

export default PipelineCanvasPanelResourceDetail;
