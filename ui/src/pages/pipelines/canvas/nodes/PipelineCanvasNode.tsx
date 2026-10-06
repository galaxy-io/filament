import { type PropsWithChildren, useCallback } from "react";

import { styled } from "@linaria/react";
import { ArrowsClockwiseIcon, GearIcon, TrashIcon } from "@phosphor-icons/react";

import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import { CONNECTOR_KIND_TO_LABEL_MAP } from "@/pages/connectors/constants";
import {
  CONNECTOR_KIND_TO_CHIP_VARIANT_MAP,
  CONNECTOR_KIND_TO_HANDLE_ID_MAP,
  CONNECTOR_KIND_TO_XYFLOW_POSITION_MAP,
} from "@/pages/pipelines/canvas/constants";
import {
  PIPELINE_CANVAS_NODE_GAP,
  PIPELINE_CANVAS_NODE_WIDTH,
} from "@/pages/pipelines/canvas/nodes/constants";
import PipelineCanvasNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeHandle";
import PipelineCanvasNodeIsland from "@/pages/pipelines/canvas/nodes/PipelineCanvasNodeIsland";

const NodeContainer = styled.div<{ $isSelected?: boolean; $width: number }>`
  width: ${({ $width }) => $width}px;

  display: flex;
  flex-direction: column;
  gap: ${PIPELINE_CANVAS_NODE_GAP}px;

  &:hover ${PipelineCanvasNodeIsland} {
    border-color: ${({ $isSelected }) =>
      $isSelected ? t.color.solid.primary.hovered : t.color.border.tertiary};
  }
`;

const ActionButton = styled.button`
  width: 20px;
  height: 20px;
  padding: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${t.color.background.primary};
  border: none;
  border-radius: 4px;
  cursor: pointer;

  transition: background-color 100ms ease;

  &:hover {
    background-color: ${t.color.background.tertiary};
  }
`;

const HeaderIsland = styled(PipelineCanvasNodeIsland)`
  display: flex;
  align-items: center;
  gap: 4px;
`;

interface PipelineCanvasNodeProps extends PropsWithChildren {
  connector: Connection["connector"];
  label: Connection["name"];
  kind: ConnectorKind;
  isConnected?: boolean;
  isSelected?: boolean;
  onRefresh?: () => void;
  onSettings?: () => void;
  onDelete?: () => void;
}

const PipelineCanvasNode = ({
  connector,
  label,
  kind,
  isConnected = false,
  isSelected = false,
  onRefresh,
  onSettings,
  onDelete,
  children,
}: PipelineCanvasNodeProps) => {
  const handleRefresh = useCallback(
    (event: React.MouseEvent) => {
      event.stopPropagation();
      onRefresh?.();
    },
    [onRefresh],
  );

  const handleDelete = useCallback(
    (event: React.MouseEvent) => {
      event.stopPropagation();
      onDelete?.();
    },
    [onDelete],
  );

  const handleSettings = useCallback(
    (event: React.MouseEvent) => {
      event.stopPropagation();
      onSettings?.();
    },
    [onSettings],
  );

  return (
    <NodeContainer $isSelected={isSelected} $width={PIPELINE_CANVAS_NODE_WIDTH}>
      <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.SPACE_BETWEEN}>
        <Chip
          label={CONNECTOR_KIND_TO_LABEL_MAP[kind]}
          variant={CONNECTOR_KIND_TO_CHIP_VARIANT_MAP[kind]}
          size={ChipSize.SMALL}
        />
        <Flex alignItems={AlignItems.CENTER} gap={4}>
          {onRefresh && (
            <ActionButton
              className="nodrag"
              onClick={handleRefresh}
              aria-label="Refresh resources"
              title="Refresh resources"
            >
              <Icon component={ArrowsClockwiseIcon} size={14} variant={IconVariant.TERTIARY} />
            </ActionButton>
          )}
          {onSettings && (
            <ActionButton
              className="nodrag"
              onClick={handleSettings}
              aria-label="Open settings"
              title="Open settings"
            >
              <Icon component={GearIcon} size={14} variant={IconVariant.TERTIARY} />
            </ActionButton>
          )}
          {onDelete && (
            <ActionButton
              className="nodrag"
              onClick={handleDelete}
              aria-label="Delete node"
              title="Delete node"
            >
              <Icon component={TrashIcon} size={14} variant={IconVariant.TERTIARY} />
            </ActionButton>
          )}
        </Flex>
      </Flex>
      <HeaderIsland $isSelected={isSelected}>
        {kind === ConnectorKind.SINK && (
          <PipelineCanvasNodeHandle
            id={CONNECTOR_KIND_TO_HANDLE_ID_MAP[kind]}
            kind={kind}
            position={CONNECTOR_KIND_TO_XYFLOW_POSITION_MAP[kind]}
            isConnected={isConnected}
          />
        )}
        <FlexItem grow={1} minWidth={0}>
          <Flex alignItems={AlignItems.CENTER} gap={8}>
            <FlexItem shrink={0}>
              <ConnectorTile connector={connector} kind={kind} />
            </FlexItem>
            <Text size={TextSize.BODY_SM} lineClamp={1}>
              {label}
            </Text>
          </Flex>
        </FlexItem>
        {kind === ConnectorKind.SOURCE && (
          <PipelineCanvasNodeHandle
            id={CONNECTOR_KIND_TO_HANDLE_ID_MAP[kind]}
            kind={kind}
            position={CONNECTOR_KIND_TO_XYFLOW_POSITION_MAP[kind]}
            isConnected={isConnected}
          />
        )}
      </HeaderIsland>
      {children}
    </NodeContainer>
  );
};

export default PipelineCanvasNode;
