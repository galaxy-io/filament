import { create } from "@bufbuild/protobuf";
import { FlowArrowIcon } from "@phosphor-icons/react";

import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { ConnectorKind, StandardSyncMode } from "@/gen/ingestion/v1/common_pb";
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
} from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { getCanvasEdgeResourceLabel } from "@/pages/pipelines/canvas/utils";
import { STANDARD_SYNC_MODE_TO_LABEL_MAP } from "@/pages/pipelines/components/create/constants";

interface PipelineCanvasPanelResourceDetailProps {
  edge: CanvasEdge;
}

const PipelineCanvasPanelResourceDetail = ({ edge }: PipelineCanvasPanelResourceDetailProps) => {
  const isReadOnly = usePipelineCanvasReadOnly();
  const { clearSelection, setShowPanel } = usePipelineCanvasSelection();
  const { setEdgeConfig } = usePipelineCanvasActions();

  const resource = getCanvasEdgeResource(edge);

  const {
    isCdc,
    isLoading,
    coveredResources,
    syncModeOptions,
    cursorOptionsByResource,
    recommendedCursorByResource,
  } = usePipelineCanvasPanelResourceOptions(edge);

  const { label: resourceLabel, isNamedResource } = getCanvasEdgeResourceLabel(
    resource,
    coveredResources.length,
  );

  const syncMode = edge.data?.standardSyncMode ?? StandardSyncMode.UNSPECIFIED;
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

  const handleSyncModeChange = (mode: StandardSyncMode) =>
    setEdgeConfig(edge.id, {
      standardSyncMode: mode,
      cursors: mode === StandardSyncMode.INCREMENTAL ? buildRecommendedCursors() : [],
    });

  const handleCursorChange = (resourceName: Resource["name"], field: ResourceColumn["name"]) =>
    setEdgeConfig(edge.id, {
      standardSyncMode: syncMode,
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

  const syncModeSelectOptions: SelectInputOption[] = syncModeOptions.map((mode) => ({
    id: String(mode),
    label: STANDARD_SYNC_MODE_TO_LABEL_MAP[mode],
    value: mode,
  }));

  return (
    <>
      <PipelineCanvasPanelHeader
        title={resourceLabel}
        icon={FlowArrowIcon}
        onBack={clearSelection}
        onClose={() => setShowPanel(false)}
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
          isEmpty={isCdc}
          emptyHeader="Managed automatically"
          emptyMessage="This connection applies inserts, updates, and deletes through CDC."
          padding="12px"
        >
          <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillWidth>
            <SelectInput
              label="Mode"
              options={syncModeSelectOptions}
              value={syncModeSelectOptions.find((option) => option.value === syncMode) ?? null}
              onChange={(option) => handleSyncModeChange(option.value as StandardSyncMode)}
              variant={InputVariant.TERTIARY}
              placeholder="Select a mode..."
              size={InputSize.LARGE}
              isDisabled={isReadOnly || isLoading}
              fillWidth
            />
            {syncMode === StandardSyncMode.INCREMENTAL &&
              coveredResources.map((resourceName) => (
                <PipelineCanvasPanelResourceCursorField
                  key={resourceName}
                  resource={resourceName}
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
