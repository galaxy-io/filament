import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";
import { ArrowsClockwiseIcon, GearSixIcon, TrashIcon } from "@phosphor-icons/react";
import { Position } from "@xyflow/react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

import { getPipelineScopedFields } from "@/components/fields/utils";

import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import { useConnectorSpec } from "@/pages/connectors/hooks/useConnectorSpec";
import { PIPELINE_NODE_GAP, PIPELINE_NODE_WIDTH } from "@/pages/pipelines/canvas/nodes/constants";
import PipelineNodeHandle from "@/pages/pipelines/canvas/nodes/PipelineNodeHandle";
import PipelineNodeIsland from "@/pages/pipelines/canvas/nodes/PipelineNodeIsland";

const NodeContainer = withTheme(styled.div<
  PropsWithTheme<{ $isSelected?: boolean; $width: number }>
>`
  width: ${({ $width }) => $width}px;

  display: flex;
  flex-direction: column;
  gap: ${PIPELINE_NODE_GAP}px;

  &:hover ${PipelineNodeIsland} {
    border-color: ${({ theme, $isSelected }) =>
      $isSelected ? theme.color.background.galaxyAlt : theme.color.border.tertiary};
  }
`);

const ActionBar = styled.div`
  display: flex;
  align-items: center;
  justify-content: space-between;
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

const HeaderIsland = styled(PipelineNodeIsland)`
  display: flex;
  align-items: center;
  gap: 4px;
`;

const HeaderContent = styled.div`
  flex: 1;
  display: flex;
  align-items: center;
  gap: 8px;
`;

interface PipelineNodeProps extends PropsWithChildren {
  connector: string;
  label: string;
  kind: ConnectorKind;
  handleId: string;
  isConnected?: boolean;
  isSelected?: boolean;
  onRefresh?: () => void;
  onDelete?: () => void;
  onConfigure?: () => void;
}

const handleActionClick = (event: React.MouseEvent, action: () => void) => {
  event.stopPropagation();
  action();
};

const PipelineNode = ({
  connector,
  label,
  kind,
  handleId,
  isConnected = false,
  isSelected = false,
  onRefresh,
  onDelete,
  onConfigure,
  children,
}: PipelineNodeProps) => {
  const connectorSpec = useConnectorSpec(connector, kind);
  const isSink = kind === ConnectorKind.SINK;
  const hasPipelineFields =
    getPipelineScopedFields(connectorSpec?.configSchema?.fields ?? []).length > 0;

  const handleSlot = (
    <PipelineNodeHandle
      id={handleId}
      kind={kind}
      position={isSink ? Position.Left : Position.Right}
      isConnected={isConnected}
    />
  );

  return (
    <NodeContainer $isSelected={isSelected} $width={PIPELINE_NODE_WIDTH}>
      <ActionBar>
        <Chip
          label={isSink ? "Sink" : "Source"}
          variant={isSink ? ChipVariant.PINK : ChipVariant.LIME}
          size={ChipSize.SMALL}
        />
        <ActionButtons>
          {onConfigure && hasPipelineFields && (
            // Propagates so React Flow also selects the node, lifting it above
            // its neighbors while the config island is open.
            <ActionButton className="nodrag" onClick={onConfigure}>
              <Icon component={GearSixIcon} size={14} variant={IconVariant.TERTIARY} />
            </ActionButton>
          )}
          {onRefresh && (
            <ActionButton
              className="nodrag"
              onClick={(event) => handleActionClick(event, onRefresh)}
            >
              <Icon component={ArrowsClockwiseIcon} size={14} variant={IconVariant.TERTIARY} />
            </ActionButton>
          )}
          {onDelete && (
            <ActionButton
              className="nodrag"
              onClick={(event) => handleActionClick(event, onDelete)}
            >
              <Icon component={TrashIcon} size={14} variant={IconVariant.TERTIARY} />
            </ActionButton>
          )}
        </ActionButtons>
      </ActionBar>
      <HeaderIsland $isSelected={isSelected}>
        {isSink && handleSlot}
        <HeaderContent>
          <ConnectorTile connector={connector} spec={connectorSpec} />
          <Text size={TextSize.BODY_SM}>{label}</Text>
        </HeaderContent>
        {!isSink && handleSlot}
      </HeaderIsland>
      {children}
    </NodeContainer>
  );
};

export default PipelineNode;
