import { styled } from "@linaria/react";

import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";

const ItemWrapper = styled.button<{ $isDisabled?: boolean }>`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 8px;
  width: 100%;

  background-color: transparent;
  border: none;
  border-radius: 4px;
  cursor: ${({ $isDisabled }) => ($isDisabled ? "not-allowed" : "pointer")};
  opacity: ${({ $isDisabled }) => ($isDisabled ? 0.5 : 1)};

  &:hover {
    background-color: ${({ $isDisabled }) =>
      $isDisabled ? "transparent" : t.color.background.tertiary};
  }
`;

const ItemName = styled.div`
  min-width: 0;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

const ChipWrapper = styled.div`
  margin-left: auto;
  flex-shrink: 0;
`;

interface PipelineCanvasConnectionSelectorItemProps {
  connection: Connection;
  isDisabled?: boolean;
  onClick: () => void;
}

const PipelineCanvasConnectionSelectorItem = ({
  connection,
  isDisabled = false,
  onClick,
}: PipelineCanvasConnectionSelectorItemProps) => {
  return (
    <ItemWrapper $isDisabled={isDisabled} onClick={isDisabled ? undefined : onClick}>
      <ConnectorTile
        connector={connection.connector}
        kind={connection.kind}
        size={ConnectorTileSize.SMALL}
      />
      <ItemName>
        <Text
          size={TextSize.BODY_SM}
          variant={isDisabled ? TextVariant.DISABLED : TextVariant.PRIMARY}
        >
          {connection.name}
        </Text>
      </ItemName>
      <ChipWrapper>
        <ConnectionKindChip kind={connection.kind} size={ChipSize.SMALL} />
      </ChipWrapper>
    </ItemWrapper>
  );
};

export default PipelineCanvasConnectionSelectorItem;
