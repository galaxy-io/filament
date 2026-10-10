import { type FC, useCallback } from "react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectionKindChip from "@/components/connections/ConnectionKindChip";
import ConnectorTile from "@/components/connections/ConnectorTile";
import { ConnectorTileSize } from "@/components/connections/types";
import { formatConnectorName } from "@/components/connections/utils";

import {
  CONNECTOR_KIND_TO_DESCRIPTION_MAP,
  CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT,
} from "@/pages/connections/constants";

interface CreateConnectionSelectorCardProps {
  connector: ConnectorSpec;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

const CreateConnectionSelectorCard: FC<CreateConnectionSelectorCardProps> = ({
  connector,
  onConnectorSelect,
}) => {
  const handleClick = useCallback(() => {
    onConnectorSelect(connector);
  }, [connector, onConnectorSelect]);

  return (
    <Widget isInteractive variant={WidgetVariant.PRIMARY} onClick={handleClick}>
      <Flex
        alignItems={AlignItems.START}
        direction={FlexDirection.COLUMN}
        gap={12}
        grow={1}
        minHeight={CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT}
      >
        <Flex
          justifyContent={JustifyContent.SPACE_BETWEEN}
          alignItems={AlignItems.CENTER}
          gap={8}
          fillWidth
        >
          <Flex gap={12} alignItems={AlignItems.CENTER} minWidth={0}>
            <ConnectorTile
              connector={connector.name}
              kind={connector.kind}
              size={ConnectorTileSize.LARGE}
            />
            <FlexItem minWidth={0} overflow="hidden">
              <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM} lineClamp={1}>
                {formatConnectorName(connector)}
              </Text>
            </FlexItem>
          </Flex>
          <ConnectionKindChip kind={connector.kind} size={ChipSize.SMALL} />
        </Flex>

        <FlexItem grow={1}>
          <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_MD}>
            {connector.description || CONNECTOR_KIND_TO_DESCRIPTION_MAP[connector.kind]}
          </Text>
        </FlexItem>
        <Button label="Connect" variant={ButtonVariant.BASE} onClick={handleClick} fillWidth />
      </Flex>
    </Widget>
  );
};

export default CreateConnectionSelectorCard;
