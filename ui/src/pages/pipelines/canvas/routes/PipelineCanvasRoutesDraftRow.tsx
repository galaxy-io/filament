import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { CheckIcon, XIcon } from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import { InputSize, InputVariant } from "@galaxy-io/dls/inputs/Input";
import SelectInput, { type SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import TextInput from "@galaxy-io/dls/inputs/TextInput";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import { TooltipPosition } from "@galaxy-io/dls/tooltip/Tooltip";
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
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";
import { getEdgeResourceStatuses, hasSiblingIncrementalRead } from "@/pages/pipelines/canvas/utils";
import {
  getCompatibleWriteModes,
  getCursorSelectOptions,
  getReadModeSelectOptions,
  getRecommendedCursor,
  getWriteModeSelectOptions,
} from "@/pages/pipelines/components/resource/utils";
import PipelineTransformFieldsIssuesChip from "@/pages/pipelines/components/transform/PipelineTransformFieldsIssuesChip";

import { isSearchMatch } from "@/utils/search";

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

const DraftLine = withTheme(styled.div<PropsWithTheme>`
  position: absolute;
  left: -${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;
  right: -${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;
  top: 50%;

  border-top: 1px dashed ${({ theme }) => theme.color.border.primary};
`);

const ControlsGroup = withTheme(styled.div<PropsWithTheme>`
  position: relative;
  z-index: 1;
  flex-shrink: 0;

  display: flex;
  align-items: center;
  gap: ${PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}px;

  background-color: ${({ theme }) => theme.color.background.secondary};
  border-radius: 5px;
`);

const searchOptions = (term: string, options: SelectInputOption[]) =>
  options.filter((option) => isSearchMatch(term, option.label));

interface PipelineCanvasRoutesDraftRowProps {
  draftState: PipelineCanvasRoutesDraftState;
}

const PipelineCanvasRoutesDraftRow = ({ draftState }: PipelineCanvasRoutesDraftRowProps) => {
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
    isLoading,
  } = options;

  const resourceOptions = useMemo<SelectInputOption[]>(
    () => (resourceNames ?? []).map((name) => ({ id: name, label: name, value: name })),
    [resourceNames],
  );
  const sinkOptions = useMemo<SelectInputOption[]>(
    () =>
      sinks.map((sink) => ({
        id: sink.nodeId,
        label: sink.label,
        value: sink.nodeId,
        icon: (
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
  const hasRecommendedCursor = getRecommendedCursor(columnsByResource.get(resource) ?? []) !== "";
  const defaultReadMode =
    hasRecommendedCursor && readModeOptions.includes(ReadMode.INCREMENTAL)
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
  const hasCursorSelect = hasReadLevers && readMode === ReadMode.INCREMENTAL;
  const statuses = getEdgeResourceStatuses(options, {
    verdict,
    readMode,
    writeMode,
    cursorsByResource: new Map(cursor !== "" ? [[resource, cursor]] : []),
  });

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
      <Widget
        variant={WidgetVariant.SECONDARY}
        padding={`0 ${PIPELINE_CANVAS_ROUTES_DRAFT_PADDING_X}px`}
        minWidth="max-content"
        fillWidth
        noHover
      >
        <FlexWrapper
          alignItems={AlignItems.CENTER}
          gap={PIPELINE_CANVAS_ROUTES_EDGE_CONTROL_GAP}
          fillWidth
          fillHeight
        >
          <FlexWrapper
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
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} isMonospace>
              /
            </Text>
            <FlexItem grow={1} minWidth={0}>
              {resourceNames ? (
                <SelectInput
                  options={resourceOptions}
                  value={resourceOptions.find((option) => option.id === resource) ?? null}
                  onChange={(option) => draftState.setResource(option.value as string)}
                  onSearch={searchOptions}
                  placeholder="Resource"
                  size={InputSize.MEDIUM}
                  isDisabled={draft.isResourceLocked}
                  fillWidth
                />
              ) : (
                <TextInput
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
          </FlexWrapper>
          <EdgeArea>
            <DraftLine />
            <ControlsGroup>
              {hasReadLevers && (
                <SelectInput
                  options={readModeSelectOptions}
                  value={readModeSelectOptions.find((option) => option.value === readMode) ?? null}
                  onChange={(option) =>
                    draftState.setConfig({ readMode: option.value as ReadMode })
                  }
                  variant={InputVariant.TERTIARY}
                  size={InputSize.SMALL}
                  placeholder="Read mode"
                  width={PIPELINE_CANVAS_ROUTES_READ_MODE_SELECT_WIDTH}
                  isDisabled={isLoading}
                />
              )}
              {hasCursorSelect && (
                <SelectInput
                  options={cursorSelectOptions}
                  value={cursorSelectOptions.find((option) => option.value === cursor) ?? null}
                  onChange={(option) => draftState.setConfig({ cursor: option.value as string })}
                  variant={InputVariant.TERTIARY}
                  size={InputSize.SMALL}
                  placeholder="Cursor"
                  width={PIPELINE_CANVAS_ROUTES_CURSOR_SELECT_WIDTH}
                  isDisabled={isLoading}
                />
              )}
            </ControlsGroup>
            <CenterSlot>
              <PipelineTransformFieldsIssuesChip
                issues={statuses.map((status) => status.message)}
                position={TooltipPosition.TOP}
              />
            </CenterSlot>
            <ControlsGroup>
              <SelectInput
                options={writeModeSelectOptions}
                value={writeModeSelectOptions.find((option) => option.value === writeMode) ?? null}
                onChange={(option) =>
                  draftState.setConfig({ writeMode: option.value as WriteMode })
                }
                variant={InputVariant.TERTIARY}
                size={InputSize.SMALL}
                placeholder="Write mode"
                width={PIPELINE_CANVAS_ROUTES_WRITE_MODE_SELECT_WIDTH}
                isDisabled={isLoading}
              />
            </ControlsGroup>
          </EdgeArea>
          <SelectInput
            options={sinkOptions}
            value={sinkOptions.find((option) => option.id === draft.sinkId) ?? null}
            onChange={(option) => draftState.setSinkId(option.value as CanvasNode["id"])}
            placeholder="Sink"
            size={InputSize.MEDIUM}
            width={PIPELINE_CANVAS_ROUTES_SINK_ISLAND_WIDTH}
            isDisabled={sinkOptions.length <= 1}
          />
          <Button
            icon={XIcon}
            ariaLabel="Cancel"
            variant={ButtonVariant.TERTIARY}
            size={ButtonSize.SMALL}
            onClick={draftState.close}
          />
          <Button
            icon={CheckIcon}
            ariaLabel="Add route"
            variant={ButtonVariant.PRIMARY_ALT}
            size={ButtonSize.SMALL}
            onClick={handleAdd}
            isDisabled={!draftState.canAdd || isLoading || statuses.length > 0}
          />
        </FlexWrapper>
      </Widget>
    </RowWrapper>
  );
};

export default PipelineCanvasRoutesDraftRow;
