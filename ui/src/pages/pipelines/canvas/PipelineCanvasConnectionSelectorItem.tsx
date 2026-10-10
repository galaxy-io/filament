import type { FC } from "react";

import { styled } from "@linaria/react";

import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { FOCUS_RING, INTERACTIVE_RESET } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import ConnectionKindChip from "@/components/connections/ConnectionKindChip";
import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";

const ItemWrapper = styled.button`
  ${INTERACTIVE_RESET}
  ${FOCUS_RING}
  display: flex;
  align-items: center;
  gap: ${t.space[8]};
  padding: ${t.space[8]};
  width: 100%;
  border-radius: ${t.radius.md};
  transition: background-color ${t.duration.fast};

  &:hover:not(:disabled) {
    background-color: ${t.color.background.hovered};
  }

  &:disabled {
    cursor: not-allowed;
    opacity: 0.5;
  }
`;

interface PipelineCanvasConnectionSelectorItemProps {
  connection: Connection;
  isDisabled?: boolean;
  onClick: () => void;
}

const PipelineCanvasConnectionSelectorItem: FC<PipelineCanvasConnectionSelectorItemProps> = ({
  connection,
  isDisabled = false,
  onClick,
}) => {
  return (
    <ItemWrapper type="button" disabled={isDisabled} onClick={onClick}>
      <ConnectorTile
        connector={connection.connector}
        kind={connection.kind}
        size={ConnectorTileSize.SMALL}
      />
      <FlexItem grow={1} minWidth={0}>
        <Text
          size={TextSize.BODY_SM}
          variant={isDisabled ? TextVariant.DISABLED : TextVariant.PRIMARY}
          lineClamp={1}
        >
          {connection.name}
        </Text>
      </FlexItem>
      <FlexItem shrink={0}>
        <ConnectionKindChip kind={connection.kind} size={ChipSize.SMALL} />
      </FlexItem>
    </ItemWrapper>
  );
};

export default PipelineCanvasConnectionSelectorItem;
