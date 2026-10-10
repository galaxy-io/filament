import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { CheckIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { InputSize } from "@galaxy-io/dls/inputs/Input";
import SelectInput, {
  SelectInputSize,
  SelectInputVariant,
  type SelectOption,
} from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import { ConnectorKind, ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import { ResourceCursorConfigSchema } from "@/gen/ingestion/v1/pipelines_pb";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import { PIPELINE_CANVAS_NODE_SINK_HANDLE_ID } from "@/pages/pipelines/canvas/constants";
import { usePipelineCanvasConnections } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasConnections";
import { usePipelineCanvasPanelResourceOptions } from "@/pages/pipelines/canvas/panel/hooks/usePipelineCanvasPanelResourceOptions";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import {
  PIPELINE_CANVAS_ROUTES_CURSOR_SELECT_WIDTH,
  PIPELINE_CANVAS_ROUTES_DRAFT_EDGE_MIN_WIDTH,
  PIPELINE_CANVAS_ROUTES_DRAFT_KEY,
  PIPELINE_CANVAS_ROUTES_DRAFT_PADDING_X,
  PIPELINE_CANVAS_ROUTES_DRAFT_PADDING_Y,
  PIPELINE_CANVAS_ROUTES_DRAFT_ROW_HEIGHT,
  PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP,
  PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_INSET,
  PIPELINE_CANVAS_ROUTES_LIST_PADDING_X,
  PIPELINE_CANVAS_ROUTES_READ_MODE_SELECT_WIDTH,
  PIPELINE_CANVAS_ROUTES_SINK_ISLAND_WIDTH,
  PIPELINE_CANVAS_ROUTES_SOURCE_ISLAND_WIDTH,
  PIPELINE_CANVAS_ROUTES_WRITE_MODE_SELECT_WIDTH,
} from "@/pages/pipelines/canvas/routes/constants";
import type { PipelineCanvasRoutesDraftState } from "@/pages/pipelines/canvas/routes/hooks/usePipelineCanvasRoutesDraft";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";
import { getEdgeResourceStatuses, hasSiblingIncrementalRead } from "@/pages/pipelines/canvas/utils";
import {
  getCompatibleWriteModes,
  getCursorSelectOptions,
  getReadModeSelectOptions,
  getRecommendedCursor,
  getWriteModeSelectOptions,
} from "@/pages/pipelines/components/resource/utils";
import PipelineTransformFieldsIssuesChip from "@/pages/pipelines/components/transform/PipelineTransformFieldsIssuesChip";

const RowWrapper = styled.div`
  height: ${PIPELINE_CANVAS_ROUTES_DRAFT_ROW_HEIGHT}px;
  padding: ${PIPELINE_CANVAS_ROUTES_DRAFT_PADDING_Y}px ${PIPELINE_CANVAS_ROUTES_LIST_PADDING_X}px;

  display: flex;
  align-items: stretch;
`;

const EdgeArea = styled.div`
  position: relative;
  flex: 1;
  min-width: ${PIPELINE_CANVAS_ROUTES_DRAFT_EDGE_MIN_WIDTH}px;
  height: 100%;
  padding: 0 ${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_INSET}px;

  display: flex;
  align-items: center;
  gap: ${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;
`;

const CenterSlot = styled.div`
  position: relative;
  z-index: 1;
  flex: 1;
  min-width: 0;

  display: flex;
  align-items: center;
  justify-content: center;
`;

const DraftLine = styled.div`
  position: absolute;
  left: -${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;
  right: -${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;
  top: 50%;

  border-top: ${HAIRLINE_WIDTH} dashed ${t.color.border.primary};
`;

const ControlsGroup = styled.div`
  position: relative;
  z-index: 1;
  flex-shrink: 0;

  display: flex;
  align-items: center;
  gap: ${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;

  background-color: ${t.color.background.secondary};
  border-radius: ${t.radius.md};
`;

interface PipelineCanvasRoutesDraftRowProps {
  draftState: PipelineCanvasRoutesDraftState;
}

const PipelineCanvasRoutesDraftRow: FC<PipelineCanvasRoutesDraftRowProps> = ({ draftState }) => {
  const { draft, resource, resourceNames, sinks, sourceNodeId } = draftState;
  const { edges } = usePipelineCanvasState();
  const connectionByNodeId = usePipelineCanvasConnections();
  const sourceConnection = sourceNodeId ? connectionByNodeId.get(sourceNodeId) : undefined;

  const draftEdge = useMemo<CanvasEdge>(
    () => ({
      id: PIPELINE_CANVAS_ROUTES_DRAFT_KEY,
      source: sourceNodeId ?? "",
      sourceHandle: resource,
      target: draft?.sinkId ?? "",
      targetHandle: PIPELINE_CANVAS_NODE_SINK_HANDLE_ID,
      data: { readMode: ReadMode.UNSPECIFIED, writeMode: WriteMode.UNSPECIFIED, cursors: [] },
    }),
    [sourceNodeId, resource, draft?.sinkId],
  );
  const options = usePipelineCanvasPanelResourceOptions(draftEdge);
  const {
    hasReadLevers,
    verdict,
    readModeOptions,
    writeModeOptions,
    effectiveReadMode,
    effectiveWriteMode,
    columnsByResource,
    cursorOptionsByResource,
    defaultCursorByResource,
    managedIncrementalResources,
    isLoading,
  } = options;

  const resourceOptions = useMemo<SelectOption[]>(
    () => (resourceNames ?? []).map((name) => ({ id: name, label: name })),
    [resourceNames],
  );
  const sinkOptions = useMemo<SelectOption[]>(
    () =>
      sinks.map((sink) => ({
        id: sink.nodeId,
        label: sink.label,
        leading: (
          <ConnectorTile
            connector={sink.connection?.connector ?? ""}
            kind={ConnectorKind.SINK}
            size={ConnectorTileSize.SMALL}
            isDeleted={!!sink.connection?.deletedAt}
          />
        ),
      })),
    [sinks],
  );

  if (!draft) return null;

  const siblingEdges = edges.filter(
    (edge) => edge.source === draftEdge.source && edge.target === draftEdge.target,
  );
  const isManagedIncremental = managedIncrementalResources.has(resource);
  const hasRecommendedCursor = getRecommendedCursor(columnsByResource.get(resource) ?? []) !== "";
  const defaultReadMode =
    (isManagedIncremental || hasRecommendedCursor) && readModeOptions.includes(ReadMode.INCREMENTAL)
      ? ReadMode.INCREMENTAL
      : effectiveReadMode;
  const readMode = draft.readMode ?? defaultReadMode;
  const routeHasIncremental =
    readMode === ReadMode.INCREMENTAL || hasSiblingIncrementalRead(edges, draftEdge);
  const compatibleWriteModes = getCompatibleWriteModes(writeModeOptions, routeHasIncremental);
  const siblingWriteMode = siblingEdges.find(
    (edge) => edge.data?.writeMode !== undefined && edge.data.writeMode !== WriteMode.UNSPECIFIED,
  )?.data?.writeMode;
  const preferredWriteMode = draft.writeMode ?? siblingWriteMode ?? effectiveWriteMode;
  const writeMode = compatibleWriteModes.includes(preferredWriteMode)
    ? preferredWriteMode
    : (compatibleWriteModes[0] ?? WriteMode.UNSPECIFIED);
  const cursor = draft.cursor ?? defaultCursorByResource[resource] ?? "";
  const hasCursorSelect =
    hasReadLevers && readMode === ReadMode.INCREMENTAL && !isManagedIncremental;
  const statuses = getEdgeResourceStatuses(options, {
    verdict,
    readMode,
    writeMode,
    cursorsByResource: new Map(cursor !== "" ? [[resource, cursor]] : []),
  });

  const blockingStatuses = statuses.filter((status) => status.isBlocking);
  const readModeSelectOptions = getReadModeSelectOptions(readModeOptions);
  const writeModeSelectOptions = getWriteModeSelectOptions(compatibleWriteModes);
  const cursorSelectOptions = getCursorSelectOptions(cursorOptionsByResource[resource] ?? []);

  const handleAdd = () =>
    draftState.add({
      readMode,
      writeMode,
      cursors:
        hasCursorSelect && cursor !== ""
          ? [create(ResourceCursorConfigSchema, { resource, field: cursor, lookbackSeconds: 0n })]
          : [],
    });

  const handleKeyDown = (event: React.KeyboardEvent<HTMLDivElement>) => {
    if (event.key !== "Escape") return;
    const isInsideOpenSelect =
      event.target instanceof Element && event.target.closest('[aria-expanded="true"]') !== null;
    if (!isInsideOpenSelect) draftState.close();
  };

  return (
    <RowWrapper onKeyDown={handleKeyDown}>
      <FlexItem grow={1} minWidth="max-content">
        <Widget variant={WidgetVariant.SECONDARY} isFlush>
          <Flex
            alignItems={AlignItems.CENTER}
            gap={PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}
            padding={[0, PIPELINE_CANVAS_ROUTES_DRAFT_PADDING_X]}
            fillWidth
            height="100%"
          >
            <Flex
              alignItems={AlignItems.CENTER}
              gap={8}
              width={PIPELINE_CANVAS_ROUTES_SOURCE_ISLAND_WIDTH}
              shrink={0}
            >
              <ConnectorTile
                connector={sourceConnection?.connector ?? ""}
                kind={ConnectorKind.SOURCE}
                size={ConnectorTileSize.SMALL}
                isDeleted={!!sourceConnection?.deletedAt}
              />
              <FlexItem shrink={0}>
                <Text size={TextSize.BODY_SM}>{sourceConnection?.name ?? sourceNodeId}</Text>
              </FlexItem>
              <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} family={FontFamily.MONO}>
                /
              </Text>
              <FlexItem grow={1} minWidth={0}>
                {resourceNames ? (
                  <SelectInput
                    ariaLabel="Resource"
                    options={resourceOptions}
                    value={resource || null}
                    onChange={(id) => {
                      if (id !== null) draftState.setResource(id);
                    }}
                    isSearchable
                    placeholder="Resource..."
                    size={SelectInputSize.MEDIUM}
                    isDisabled={draft.isResourceLocked}
                    fillWidth
                  />
                ) : (
                  <TextInput
                    ariaLabel="Resource"
                    value={resource}
                    onChange={draftState.setResource}
                    placeholder="orders.>"
                    size={InputSize.MEDIUM}
                    isDisabled={draft.isResourceLocked}
                    autoFocus
                    fillWidth
                  />
                )}
              </FlexItem>
            </Flex>
            <EdgeArea>
              <DraftLine />
              <ControlsGroup>
                {hasReadLevers && (
                  <Box width={PIPELINE_CANVAS_ROUTES_READ_MODE_SELECT_WIDTH}>
                    <SelectInput
                      ariaLabel="Read mode"
                      fillWidth
                      options={readModeSelectOptions}
                      value={String(readMode)}
                      onChange={(id) => {
                        if (id !== null) draftState.setConfig({ readMode: Number(id) as ReadMode });
                      }}
                      variant={SelectInputVariant.TERTIARY}
                      size={SelectInputSize.SMALL}
                      placeholder="Read mode..."
                      isDisabled={isLoading}
                    />
                  </Box>
                )}
                {hasCursorSelect && (
                  <Box width={PIPELINE_CANVAS_ROUTES_CURSOR_SELECT_WIDTH}>
                    <SelectInput
                      ariaLabel="Cursor"
                      fillWidth
                      options={cursorSelectOptions}
                      value={cursor || null}
                      onChange={(id) => {
                        if (id !== null) draftState.setConfig({ cursor: id });
                      }}
                      variant={SelectInputVariant.TERTIARY}
                      size={SelectInputSize.SMALL}
                      placeholder="Cursor..."
                      isDisabled={isLoading}
                    />
                  </Box>
                )}
              </ControlsGroup>
              <CenterSlot>
                <PipelineTransformFieldsIssuesChip
                  issues={blockingStatuses.map((status) => status.message)}
                  warnings={statuses
                    .filter((status) => !status.isBlocking)
                    .map((status) => status.message)}
                />
              </CenterSlot>
              <ControlsGroup>
                <Box width={PIPELINE_CANVAS_ROUTES_WRITE_MODE_SELECT_WIDTH}>
                  <SelectInput
                    ariaLabel="Write mode"
                    fillWidth
                    options={writeModeSelectOptions}
                    value={String(writeMode)}
                    onChange={(id) => {
                      if (id !== null) draftState.setConfig({ writeMode: Number(id) as WriteMode });
                    }}
                    variant={SelectInputVariant.TERTIARY}
                    size={SelectInputSize.SMALL}
                    placeholder="Write mode..."
                    isDisabled={isLoading}
                  />
                </Box>
              </ControlsGroup>
            </EdgeArea>
            <Box width={PIPELINE_CANVAS_ROUTES_SINK_ISLAND_WIDTH}>
              <SelectInput
                ariaLabel="Sink"
                fillWidth
                options={sinkOptions}
                value={draft.sinkId || null}
                onChange={(id) => {
                  if (id !== null) draftState.setSinkId(id);
                }}
                placeholder="Sink..."
                size={SelectInputSize.MEDIUM}
                isDisabled={sinkOptions.length <= 1}
              />
            </Box>
            <Button
              icon={XIcon}
              ariaLabel="Cancel"
              tooltip="Cancel"
              variant={ButtonVariant.TERTIARY}
              size={ButtonSize.SMALL}
              onClick={draftState.close}
            />
            <Button
              icon={CheckIcon}
              ariaLabel="Add route"
              tooltip="Add route"
              variant={ButtonVariant.PRIMARY}
              size={ButtonSize.SMALL}
              onClick={handleAdd}
              isDisabled={!draftState.canAdd || isLoading || blockingStatuses.length > 0}
            />
          </Flex>
        </Widget>
      </FlexItem>
    </RowWrapper>
  );
};

export default PipelineCanvasRoutesDraftRow;
