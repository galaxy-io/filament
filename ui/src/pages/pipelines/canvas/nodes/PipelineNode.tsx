import type { PropsWithChildren } from "react";

import { Position } from "@xyflow/react";
import { styled } from "@linaria/react";
import { ArrowsClockwiseIcon, TrashIcon } from "@phosphor-icons/react";

import Chip, { ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ProviderKind } from "@/gen/ingestion/v1/common_pb";

import {
  PIPELINE_NODE_WIDTH,
  PIPELINE_NODE_SINK_WIDTH,
  PIPELINE_NODE_PADDING,
  PIPELINE_NODE_BORDER_RADIUS,
  PIPELINE_NODE_HANDLE_SLOT_SIZE,
} from "@/pages/pipelines/canvas/constants";
import PipelineNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineNodeHandle";
import ProviderTile, { ProviderTileSize } from "@/pages/providers/components/ProviderTile";

// Island base styles - used by header and child islands
export const Island = withTheme(styled.div<PropsWithTheme<{ $isSelected?: boolean }>>`
  padding: ${PIPELINE_NODE_PADDING}px;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: 1px solid ${({ theme, $isSelected }) =>
    $isSelected ? theme.color.background.galaxy : theme.color.border.primary};
  border-radius: ${PIPELINE_NODE_BORDER_RADIUS}px;

  transition: border-color 100ms ease;
`);

// Container that propagates hover to all islands
const NodeContainer = withTheme(styled.div<PropsWithTheme<{ $isSelected?: boolean; $width: number }>>`
  width: ${({ $width }) => $width}px;

  &:hover ${Island} {
    border-color: ${({ theme, $isSelected }) =>
      $isSelected ? theme.color.background.galaxyAlt : theme.color.border.tertiary};
  }
`);

const ActionBar = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 8px;
`;

const ActionButtons = styled.div`
  display: flex;
  align-items: center;
  gap: 4px;
`;

const ActionButton = withTheme(styled.button<PropsWithTheme>`
  width: 20px;
  height: 20px;
  padding: 0;

  display: flex;
  align-items: center;
  justify-content: center;

  background-color: ${({ theme }) => theme.color.background.primary};
  border: none;
  border-radius: 4px;
  cursor: pointer;

  transition: background-color 100ms ease;

  &:hover {
    background-color: ${({ theme }) => theme.color.background.tertiary};
  }
`);

const HeaderIsland = styled(Island)`
  display: flex;
  align-items: center;
  gap: 6px;
`;

const HeaderContent = styled.div`
  flex: 1;
  display: flex;
  align-items: center;
  gap: 10px;
`;

// 24x24 container that centers the handle
const HandleSlot = styled.div`
  width: ${PIPELINE_NODE_HANDLE_SLOT_SIZE}px;
  height: ${PIPELINE_NODE_HANDLE_SLOT_SIZE}px;

  display: flex;
  align-items: center;
  justify-content: center;
`;

interface PipelineNodeProps extends PropsWithChildren {
  provider: string;
  kind: ProviderKind;
  handleId: string;
  isConnected?: boolean;
  isSelected?: boolean;
  onRefresh?: () => void;
  onDelete?: () => void;
}

const PipelineNode = ({
  provider,
  kind,
  handleId,
  isConnected = false,
  isSelected = false,
  onRefresh,
  onDelete,
  children,
}: PipelineNodeProps) => {
  const handlePosition = kind === ProviderKind.SINK ? Position.Left : Position.Right;
  const nodeWidth = kind === ProviderKind.SINK ? PIPELINE_NODE_SINK_WIDTH : PIPELINE_NODE_WIDTH;

  const handleSlot = (
    <HandleSlot>
      <PipelineNodeHandle
        id={handleId}
        kind={kind}
        position={handlePosition}
        isConnected={isConnected}
      />
    </HandleSlot>
  );

  return (
    <NodeContainer $isSelected={isSelected} $width={nodeWidth}>
      <ActionBar>
        <Chip
          label={kind === ProviderKind.SOURCE ? "Source" : "Sink"}
          variant={kind === ProviderKind.SOURCE ? ChipVariant.LIME : ChipVariant.PINK}
        />
        <ActionButtons>
          <ActionButton onClick={onRefresh}>
            <Icon component={ArrowsClockwiseIcon} size={14} variant={IconVariant.TERTIARY} />
          </ActionButton>
          <ActionButton onClick={onDelete}>
            <Icon component={TrashIcon} size={14} variant={IconVariant.TERTIARY} />
          </ActionButton>
        </ActionButtons>
      </ActionBar>
      <HeaderIsland $isSelected={isSelected}>
        {kind === ProviderKind.SINK && handleSlot}
        <HeaderContent>
          <ProviderTile provider={provider} size={ProviderTileSize.SMALL} />
          <Text size={TextSize.BODY_SM}>{provider}</Text>
        </HeaderContent>
        {kind === ProviderKind.SOURCE && handleSlot}
      </HeaderIsland>
      {children}
    </NodeContainer>
  );
};

export default PipelineNode;
