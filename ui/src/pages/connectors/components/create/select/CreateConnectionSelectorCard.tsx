import { useCallback } from "react";

import Button from "@galaxy-io/dls/buttons/Button";
import { ChipSize } from "@galaxy-io/dls/chips/Chip";
import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { ConnectorSpec } from "@/gen/ingestion/v1/connectors_pb";

import ConnectionKindChip from "@/pages/connectors/components/ConnectionKindChip";
import ConnectorTile from "@/pages/connectors/components/ConnectorTile";
import {
  CONNECTOR_KIND_TO_DESCRIPTION_MAP,
  CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT,
} from "@/pages/connectors/constants";

import ConnectorMaturityIcon from "../../ConnectorMaturityIcon";

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
    <Widget
      fillWidth
      minHeight={CREATE_CONNECTION_SELECTOR_CARD_MIN_HEIGHT}
      variant={WidgetVariant.PRIMARY}
      onClick={handleClick}
    >
      <FlexWrapper direction={FlexDirection.COLUMN} gap={12} fillHeight>
        <FlexWrapper
          justifyContent={JustifyContent.SPACE_BETWEEN}
          alignItems={AlignItems.CENTER}
          fillWidth
        >
          <FlexWrapper gap={8} alignItems={AlignItems.CENTER}>
            <ConnectorTile connector={connector.name} kind={connector.kind} />
            <Text weight={TextWeight.MEDIUM}>{connector.displayName || connector.name}</Text>
          </FlexWrapper>
          <FlexWrapper alignItems={AlignItems.CENTER} gap={8}>
            <ConnectorMaturityIcon maturity={connector.maturity} />
            <ConnectionKindChip kind={connector.kind} size={ChipSize.SMALL} />
          </FlexWrapper>
        </FlexWrapper>

        <FlexItem grow={1}>
          <Text variant={TextVariant.TERTIARY} size={TextSize.BODY_SM}>
            {connector.description || CONNECTOR_KIND_TO_DESCRIPTION_MAP[connector.kind]}
          </Text>
        </FlexItem>
        <Button label="Connect" onClick={handleClick} fillWidth />
      </FlexWrapper>
    </Widget>
  );
};

export default CreateConnectionSelectorCard;
