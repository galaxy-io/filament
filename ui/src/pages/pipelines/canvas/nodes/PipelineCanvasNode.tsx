import { type PropsWithChildren, useCallback } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { ArrowsClockwiseIcon, GearSixIcon, TrashIcon } from "@phosphor-icons/react";

import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { ConnectorKind } from "@/gen/ingestion/v1/common_pb";
import { GetConnectorRequestSchema } from "@/gen/ingestion/v1/providers_pb";

import { getPipelineScopedFields } from "@/components/fields/utils";

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

import { useGetConnectorQuery } from "@/api/queries/connectors";

const NodeContainer = withTheme(styled.div<
  PropsWithTheme<{ $isSelected?: boolean; $width: number }>
>`
  width: ${({ $width }) => $width}px;

  display: flex;
  flex-direction: column;
  gap: ${PIPELINE_CANVAS_NODE_GAP}px;

  &:hover ${PipelineCanvasNodeIsland} {
    border-color: ${({ theme, $isSelected }) =>
      $isSelected ? theme.color.background.galaxyAlt : theme.color.border.tertiary};
  }
`);

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

const HeaderIsland = styled(PipelineCanvasNodeIsland)`
  display: flex;
  align-items: center;
  gap: 4px;
`;

interface PipelineCanvasNodeProps extends PropsWithChildren {
  connector: string;
  label: string;
  kind: ConnectorKind;
  isConnected?: boolean;
  isSelected?: boolean;
  onRefresh?: () => void;
  onDelete?: () => void;
  onConfigure?: () => void;
}

const PipelineCanvasNode = ({
  connector,
  label,
  kind,
  isConnected = false,
  isSelected = false,
  onRefresh,
  onDelete,
  onConfigure,
  children,
}: PipelineCanvasNodeProps) => {
  const { data } = useGetConnectorQuery({
    input: create(GetConnectorRequestSchema, { connector, kind }),
    options: { enabled: !!connector && !!kind },
  });
  const connectorSpec = data?.connector;

  const hasPipelineFields =
    getPipelineScopedFields(connectorSpec?.configSchema?.fields ?? []).length > 0;

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

  return (
    <NodeContainer $isSelected={isSelected} $width={PIPELINE_CANVAS_NODE_WIDTH}>
      <FlexWrapper alignItems={AlignItems.CENTER} justifyContent={JustifyContent.SPACE_BETWEEN}>
        <Chip
          label={CONNECTOR_KIND_TO_LABEL_MAP[kind]}
          variant={CONNECTOR_KIND_TO_CHIP_VARIANT_MAP[kind]}
          size={ChipSize.SMALL}
        />
        <FlexWrapper alignItems={AlignItems.CENTER} gap={4}>
          {onConfigure && hasPipelineFields && (
            <ActionButton className="nodrag" onClick={onConfigure}>
              <Icon component={GearSixIcon} size={14} variant={IconVariant.TERTIARY} />
            </ActionButton>
          )}
          {onRefresh && (
            <ActionButton className="nodrag" onClick={handleRefresh}>
              <Icon component={ArrowsClockwiseIcon} size={14} variant={IconVariant.TERTIARY} />
            </ActionButton>
          )}
          {onDelete && (
            <ActionButton className="nodrag" onClick={handleDelete}>
              <Icon component={TrashIcon} size={14} variant={IconVariant.TERTIARY} />
            </ActionButton>
          )}
        </FlexWrapper>
      </FlexWrapper>
      <HeaderIsland $isSelected={isSelected}>
        {kind === ConnectorKind.SINK && (
          <PipelineCanvasNodeHandle
            id={CONNECTOR_KIND_TO_HANDLE_ID_MAP[kind]}
            kind={kind}
            position={CONNECTOR_KIND_TO_XYFLOW_POSITION_MAP[kind]}
            isConnected={isConnected}
          />
        )}
        <FlexItem grow={1} shrink={0}>
          <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
            <ConnectorTile connector={connector} kind={kind} />
            <Text size={TextSize.BODY_SM}>{label}</Text>
          </FlexWrapper>
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
