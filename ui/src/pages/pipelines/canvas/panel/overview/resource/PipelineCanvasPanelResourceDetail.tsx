import { type FC, Suspense } from "react";

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

import { ConnectorKind, ReadMode, type WriteMode } from "@/gen/ingestion/v1/common_pb";

import ConnectionDrawerKeyValueRow from "@/pages/connectors/components/drawer/ConnectionDrawerKeyValueRow";
import ConnectionDrawerList from "@/pages/connectors/components/drawer/ConnectionDrawerList";
import { getCanvasEdgeResource } from "@/pages/pipelines/canvas/graph/serialize";
import { usePipelineCanvasEdgeConfig } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasEdgeConfig";
import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import { usePipelineCanvasPanelResourceOptions } from "@/pages/pipelines/canvas/panel/hooks/usePipelineCanvasPanelResourceOptions";
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
  getCanvasEdgeResourceLabel,
  getDefaultDestinationResource,
  getEdgeResourceStatuses,
} from "@/pages/pipelines/canvas/utils";
import { PipelineResourceStatusField } from "@/pages/pipelines/components/resource/types";
import PipelineTransformFieldsProvider from "@/pages/pipelines/components/transform/PipelineTransformFieldsProvider";
import type { TransformDefinition } from "@/pages/pipelines/components/transform/types";

interface PipelineCanvasPanelResourceDetailProps {
  edge: CanvasEdge;
}

const PipelineCanvasPanelResourceDetail: FC<PipelineCanvasPanelResourceDetailProps> = ({
  edge,
}) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const { clearSelection, setShowPanel } = usePipelineCanvasSelection();
  const { setEdgeConfig, applyEdgeChanges } = usePipelineCanvasActions();

  const resource = getCanvasEdgeResource(edge);

  const options = usePipelineCanvasPanelResourceOptions(edge);
  const {
    isContinuous,
    hasReadLevers,
    isTransformable,
    isLoading,
    isLoadingColumns,
    sourceConnectionId,
    coveredResources,
    columnsByResource,
    verdict,
    readModeOptions,
    writeModeOptions,
    effectiveReadMode,
    effectiveWriteMode,
    cursorOptionsByResource,
    defaultCursorByResource,
    managedIncrementalResources,
  } = options;

  const { label: resourceLabel, isNamedResource } = getCanvasEdgeResourceLabel(
    resource,
    coveredResources.length,
  );

  const {
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
  } = usePipelineCanvasEdgeConfig(edge, {
    readModeOptions,
    writeModeOptions,
    effectiveReadMode,
    effectiveWriteMode,
    coveredResources,
    defaultCursorByResource,
  });
  const destinationResource = edge.data?.destinationResource ?? "";
  const statuses = getEdgeResourceStatuses(options, {
    verdict,
    readMode,
    writeMode,
    cursorsByResource,
  });
  const getFieldError = (field: PipelineResourceStatusField, resourceName?: string) =>
    statuses.find(
      (status) =>
        status.isBlocking &&
        status.field === field &&
        (resourceName === undefined || status.resource === resourceName),
    )?.message;

  const handleTransformChange = (transform: TransformDefinition | undefined) =>
    setEdgeConfig(edge.id, {
      readMode: configuredReadMode,
      writeMode: configuredWriteMode,
      cursors,
      transform,
    });
  const handleDestinationChange = (value: string) =>
    setEdgeConfig(edge.id, { readMode, writeMode, cursors, destinationResource: value.trim() });

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
                    error={getFieldError(PipelineResourceStatusField.READ_MODE)}
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
                  error={getFieldError(PipelineResourceStatusField.WRITE_MODE)}
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
                        error={getFieldError(PipelineResourceStatusField.CURSOR, resourceName)}
                        onChange={(field) => handleCursorChange(resourceName, field)}
                      />
                    ),
                  )}
              </Flex>
            </PipelineCanvasPanelSection>
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
                      resources={coveredResources}
                      columnsByResource={columnsByResource}
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
