import { styled } from "@linaria/react";

import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import {
  CONNECTOR_KIND_TO_CHIP_VARIANT_MAP,
  CONNECTOR_KIND_TO_LABEL_MAP,
} from "@/pages/connectors/constants";

import type { ConnectorKind } from "@/gen/ingestion/v1/common_pb";

const ItemWrapper = withTheme(styled.button<PropsWithTheme<{ $isDisabled?: boolean }>>`
  display: flex;
  align-items: center;
  gap: 8px;
  padding: 4px;
  width: 100%;

  background-color: transparent;
  border: none;
  border-radius: 4px;
  cursor: ${({ $isDisabled }) => ($isDisabled ? "not-allowed" : "pointer")};
  opacity: ${({ $isDisabled }) => ($isDisabled ? 0.5 : 1)};

  transition: all 75ms ease-in-out;

  &:hover {
    background-color: ${({ theme, $isDisabled }) =>
      $isDisabled ? "transparent" : theme.color.background.tertiary};
    padding: ${({ $isDisabled }) => ($isDisabled ? "4px" : "4px 8px")};
  }
`);

const ItemName = styled.div`
  flex: 1;
  min-width: 0;
  text-align: left;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
`;

interface EditWidgetSelectorItemProps {
  name: string;
  connector: string;
  kind: ConnectorKind;
  isDisabled?: boolean;
  onClick: () => void;
}

const EditWidgetSelectorItem = ({
  name,
  connector,
  kind,
  isDisabled = false,
  onClick,
}: EditWidgetSelectorItemProps) => {
  return (
    <ItemWrapper $isDisabled={isDisabled} onClick={isDisabled ? undefined : onClick}>
      <ConnectorTile connector={connector} size={ConnectorTileSize.SMALL} />
      <ItemName>
        <Text
          size={TextSize.BODY_SM}
          variant={isDisabled ? TextVariant.DISABLED : TextVariant.PRIMARY}
        >
          {name}
        </Text>
      </ItemName>
      <FlexItem shrink={0}>
        <Chip
          label={CONNECTOR_KIND_TO_LABEL_MAP[kind]}
          variant={CONNECTOR_KIND_TO_CHIP_VARIANT_MAP[kind]}
          size={ChipSize.SMALL}
        />
      </FlexItem>
    </ItemWrapper>
  );
};

export default EditWidgetSelectorItem;
