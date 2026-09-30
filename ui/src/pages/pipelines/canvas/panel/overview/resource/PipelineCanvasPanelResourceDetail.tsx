import { Suspense } from "react";

import { FlowArrowIcon } from "@phosphor-icons/react";
import { CatchBoundary } from "@tanstack/react-router";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

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
import PipelineCanvasPanelBody from "@/pages/pipelines/canvas/panel/PipelineCanvasPanelBody";
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

const PipelineCanvasPanelResourceDetail = ({ edge }: PipelineCanvasPanelResourceDetailProps) => {
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
        status.field === field && (resourceName === undefined || status.resource === resourceName),
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
            {hasReadLevers && (
              <SelectInput
                label="Read mode"
                options={readModeSelectOptions}
                value={readModeSelectOptions.find((option) => option.value === readMode) ?? null}
                onChange={(option) => handleReadModeChange(option.value as ReadMode)}
                variant={InputVariant.TERTIARY}
                placeholder="Select a read mode..."
                size={InputSize.LARGE}
                isDisabled={isReadOnly || isLoading}
                error={getFieldError(PipelineResourceStatusField.READ_MODE)}
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
              error={getFieldError(PipelineResourceStatusField.WRITE_MODE)}
              fillWidth
            />
            {isContinuous && isNamedResource && (
              <TextInput
                label="Destination"
                value={destinationResource}
                onChange={handleDestinationChange}
                placeholder={getDefaultDestinationResource(resource)}
                variant={InputVariant.TERTIARY}
                size={InputSize.LARGE}
                isDisabled={isReadOnly}
                fillWidth
              />
            )}
            {hasReadLevers &&
              readMode === ReadMode.INCREMENTAL &&
              coveredResources.map((resourceName) => (
                <PipelineCanvasPanelResourceCursorField
                  key={resourceName}
                  value={cursorsByResource.get(resourceName) ?? ""}
                  options={cursorOptionsByResource[resourceName] ?? []}
                  isDisabled={isReadOnly || isLoading}
                  error={getFieldError(PipelineResourceStatusField.CURSOR, resourceName)}
                  onChange={(field) => handleCursorChange(resourceName, field)}
                />
              ))}
          </FlexWrapper>
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
      </PipelineCanvasPanelBody>
    </>
  );
};

export default PipelineCanvasPanelResourceDetail;
