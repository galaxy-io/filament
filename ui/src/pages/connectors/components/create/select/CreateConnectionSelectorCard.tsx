import { useCallback } from "react";

import Button from "@galaxy-io/dls/buttons/Button";
import Chip, { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import ConnectorTile, { ConnectorTileSize } from "@/pages/connectors/components/ConnectorTile";
import {
  CONNECTOR_KIND_TO_CHIP_VARIANT_MAP,
  CONNECTOR_KIND_TO_DESCRIPTION_MAP,
  CONNECTOR_KIND_TO_LABEL_MAP,
} from "@/pages/connectors/constants";

import type { ConnectorSpec } from "@/gen/ingestion/v1/providers_pb";

interface CreateConnectionSelectorCardProps {
  connector: ConnectorSpec;
  onConnectorSelect: (connector: ConnectorSpec) => void;
}

const CreateConnectionSelectorCard = ({
  connector,
  onConnectorSelect,
}: CreateConnectionSelectorCardProps) => {
  const handleClick = useCallback(() => {
    onConnectorSelect(connector);
  }, [connector, onConnectorSelect]);

  return (
    <Widget fillWidth minHeight={150} variant={WidgetVariant.TERTIARY} onClick={handleClick}>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillHeight>
        <FlexWrapper
          justifyContent={JustifyContent.SPACE_BETWEEN}
          alignItems={AlignItems.CENTER}
          fillWidth
        >
          <FlexWrapper gap={8} alignItems={AlignItems.CENTER}>
            <ConnectorTile connector={connector.name} size={ConnectorTileSize.SMALL} />
            <Text weight={TextWeight.MEDIUM}>{connector.displayName || connector.name}</Text>
          </FlexWrapper>
          <Chip
            label={CONNECTOR_KIND_TO_LABEL_MAP[connector.kind]}
            variant={CONNECTOR_KIND_TO_CHIP_VARIANT_MAP[connector.kind]}
            size={ChipSize.SMALL}
          />
        </FlexWrapper>

        <FlexItem grow={1}>
          <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_SM}>
            {CONNECTOR_KIND_TO_DESCRIPTION_MAP[connector.kind]}
          </Text>
        </FlexItem>

        <Button label="Connect" onClick={handleClick} fillWidth />
      </FlexWrapper>
    </Widget>
  );
};

export default CreateConnectionSelectorCard;
