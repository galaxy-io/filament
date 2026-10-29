import { type FC, Suspense, useMemo } from "react";

import { FlowArrowIcon } from "@phosphor-icons/react";
import { CatchBoundary } from "@tanstack/react-router";

import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { SelectInputVariant } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { ConnectorKind, ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";

import KeyValueList from "@/components/KeyValueList";
import KeyValueListRow from "@/components/KeyValueListRow";

import { usePipelineCanvasEdgeConfig } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasEdgeConfig";
import { usePipelineCanvasPanelResourceOptions } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasPanelResourceOptions";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasPanelResourceColumnsSection from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceColumnsSection";
import PipelineCanvasPanelResourceCursorField from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceCursorField";
import PipelineCanvasPanelResourceEndpoint from "@/pages/pipelines/canvas/panel/overview/resource/PipelineCanvasPanelResourceEndpoint";
import PipelineCanvasPanelResourceTransformSection from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformSection";
import PipelineCanvasPanelResourceTransformSectionError from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformSectionError";
import PipelineCanvasPanelResourceTransformSectionPending from "@/pages/pipelines/canvas/panel/overview/resource/transform/PipelineCanvasPanelResourceTransformSectionPending";
import PipelineCanvasPanelHeader from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelHeader";
import PipelineCanvasPanelSection from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelSection";
import {
  usePipelineCanvasActions,
  usePipelineCanvasReadOnly,
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import {
  getDefaultDestinationResource,
  getEdgeResourceStatus,
} from "@/pages/pipelines/canvas/utils";
import { PipelineResourceStatusField } from "@/pages/pipelines/components/resource/types";
import {
  createResourceColumnSelections,
  getEffectiveColumnSelection,
  getLockedColumnNames,
} from "@/pages/pipelines/components/resource/utils";
import PipelineTransformFieldsProvider from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import type { TransformDefinition } from "@/pages/pipelines/components/transform/types";

import { getEnumSelectValue, mapOptionIdToEnum } from "@/utils/select";

const NO_COLUMNS: ResourceColumn[] = [];

interface PipelineCanvasPanelResourceDetailProps {
  edge: CanvasEdge;
}

const PipelineCanvasPanelResourceDetail: FC<PipelineCanvasPanelResourceDetailProps> = ({
  edge,
}) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const { clearSelection, setShowPanel } = usePipelineCanvasSelection();
  const { setEdgeConfig, applyEdgeChanges } = usePipelineCanvasActions();

  const options = usePipelineCanvasPanelResourceOptions(edge);
  const {
    isContinuous,
    hasReadLevers,
    isTransformable,
    isLoading,
    isLoadingColumns,
    sourceConnectionId,
    resource,
    columns,
    verdict,
    readModeOptions,
    writeModeOptions,
    effectiveReadMode,
    effectiveWriteMode,
    cursorOptions,
    defaultCursor,
    isManagedIncremental,
    isColumnSelectable,
  } = options;

  const {
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
  } = usePipelineCanvasEdgeConfig(edge, {
    readModeOptions,
    writeModeOptions,
    effectiveReadMode,
    effectiveWriteMode,
    resource,
    defaultCursor,
    hasReadLevers,
  });
  const destinationResource = edge.data?.destinationResource ?? "";
  const status = getEdgeResourceStatus(options, { verdict, readMode, writeMode, cursor });
  const getFieldError = (field: PipelineResourceStatusField) =>
    status?.isBlocking && status.field === field ? status.message : undefined;

  const handleTransformChange = (transform: TransformDefinition | undefined) =>
    setEdgeConfig(edge.id, {
      readMode: configuredReadMode,
      writeMode: configuredWriteMode,
      cursors,
      transform,
    });
  const handleDestinationChange = (value: string) =>
    setEdgeConfig(edge.id, { readMode, writeMode, cursors, destinationResource: value });

  const selection = edge.data?.columns?.find((candidate) => candidate.resource === resource);
  const lockedColumnNames = useMemo(
    () =>
      getLockedColumnNames({
        primaryKey: (columns ?? NO_COLUMNS)
          .filter((column) => column.isPrimaryKey)
          .map((column) => column.name),
        cursor: cursor || defaultCursor,
        isCursorLocked: hasReadLevers && readMode === ReadMode.INCREMENTAL && !isManagedIncremental,
      }),
    [columns, cursor, defaultCursor, hasReadLevers, readMode, isManagedIncremental],
  );
  const selectedColumnNames = useMemo(
    () =>
      getEffectiveColumnSelection({
        stored: selection?.columns,
        columns,
        locked: lockedColumnNames,
      }),
    [selection, columns, lockedColumnNames],
  );
  const selectedColumns = useMemo(
    () => (columns ?? NO_COLUMNS).filter((column) => selectedColumnNames.includes(column.name)),
    [columns, selectedColumnNames],
  );
  const columnsError = verdict?.errors.find(
    (error) => error.field === PipelineResourceStatusField.COLUMNS,
  )?.message;
  const handleColumnsChange = (next: ResourceColumn["name"][]) =>
    setEdgeConfig(edge.id, {
      readMode: configuredReadMode,
      writeMode: configuredWriteMode,
      cursors,
      columns: createResourceColumnSelections({
        resource,
        selected: next,
        columns: columns ?? NO_COLUMNS,
      }),
    });

  return (
    <>
      <PipelineCanvasPanelHeader
        title={resource}
        icon={FlowArrowIcon}
        onBack={clearSelection}
        onClose={() => setShowPanel(false)}
        hasDivider
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
            <KeyValueList variant={BoxVariant.SECONDARY}>
              <KeyValueListRow
                label="Source"
                value={
                  <PipelineCanvasPanelResourceEndpoint
                    nodeId={edge.source}
                    kind={ConnectorKind.SOURCE}
                  />
                }
              />
              <KeyValueListRow
                label="Resource"
                value={
                  <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
                    {resource}
                  </Text>
                }
              />
              <KeyValueListRow
                label="Sink"
                value={
                  <PipelineCanvasPanelResourceEndpoint
                    nodeId={edge.target}
                    kind={ConnectorKind.SINK}
                  />
                }
              />
            </KeyValueList>
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
                    value={getEnumSelectValue(ReadMode, readMode)}
                    onChange={(id) => {
                      if (id !== null) handleReadModeChange(mapOptionIdToEnum(ReadMode, id));
                    }}
                    placeholder="Select a read mode..."
                    isDisabled={isReadOnly || isLoading}
                    error={getFieldError(PipelineResourceStatusField.READ_MODE)}
                    variant={SelectInputVariant.TERTIARY}
                    fillWidth
                  />
                )}
                {hasReadLevers &&
                  readMode === ReadMode.INCREMENTAL &&
                  (isManagedIncremental ? (
                    <Text size={TextSize.BODY_SM}>Incremental state is managed by the source.</Text>
                  ) : (
                    <PipelineCanvasPanelResourceCursorField
                      value={cursor ?? ""}
                      options={cursorOptions}
                      isDisabled={isReadOnly || isLoading}
                      error={getFieldError(PipelineResourceStatusField.CURSOR)}
                      onChange={handleCursorChange}
                    />
                  ))}
                <SelectInput
                  label="Write mode"
                  options={writeModeSelectOptions}
                  value={getEnumSelectValue(WriteMode, writeMode)}
                  onChange={(id) => {
                    if (id !== null) handleWriteModeChange(mapOptionIdToEnum(WriteMode, id));
                  }}
                  placeholder="Select a write mode..."
                  isDisabled={isReadOnly || isLoading}
                  error={getFieldError(PipelineResourceStatusField.WRITE_MODE)}
                  variant={SelectInputVariant.TERTIARY}
                  fillWidth
                />
                {isContinuous && (
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
              </Flex>
            </PipelineCanvasPanelSection>
            {isColumnSelectable && (
              <PipelineCanvasPanelResourceColumnsSection
                columns={columns ?? NO_COLUMNS}
                selected={selectedColumnNames}
                locked={lockedColumnNames}
                isDisabled={isReadOnly || isLoading}
                error={columnsError}
                onChange={handleColumnsChange}
              />
            )}
            {isTransformable &&
              (isLoadingColumns ? (
                <PipelineCanvasPanelResourceTransformSectionPending />
              ) : (
                <CatchBoundary
                  getResetKey={() => edge.id}
                  errorComponent={PipelineCanvasPanelResourceTransformSectionError}
                >
                  <Suspense fallback={<PipelineCanvasPanelResourceTransformSectionPending />}>
                    <PipelineTransformFieldsProvider
                      definition={edge.data?.transform}
                      onChange={handleTransformChange}
                      resource={resource}
                      columns={selectedColumns}
                      sourceConnectionId={sourceConnectionId}
                      isReadOnly={isReadOnly}
                    >
                      <PipelineCanvasPanelResourceTransformSection />
                    </PipelineTransformFieldsProvider>
                  </Suspense>
                </CatchBoundary>
              ))}
          </Flex>
        </ScrollArea>
      </FlexItem>
    </>
  );
};

export default PipelineCanvasPanelResourceDetail;
