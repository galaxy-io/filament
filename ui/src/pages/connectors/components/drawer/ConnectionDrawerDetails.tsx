import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import { useConnectorSpec } from "@/pages/connectors/hooks";

import type { Connection } from "@/gen/ingestion/v1/connections_pb";

interface ConnectionDrawerDetailsProps {
  connection: Connection;
}

const ConnectionDrawerDetails = ({ connection }: ConnectionDrawerDetailsProps) => {
  const connector = useConnectorSpec(connection.connector, connection.kind);

  return (
    <Widget variant={WidgetVariant.BASE} fillWidth noHover padding="12px">
      <FlexWrapper fillWidth direction={FlexDirection.COLUMN} gap={FlexGap.SMALL}>
        <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
          Details
        </Text>
        <HorizontalDivider />
        <FlexWrapper
          fillWidth
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
        >
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            Connection ID
          </Text>
          <Text size={TextSize.BODY_SM}>{connection.id}</Text>
        </FlexWrapper>
        <FlexWrapper
          fillWidth
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
        >
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            Connector
          </Text>
          <Text size={TextSize.BODY_SM}>{connector?.displayName || connection.connector}</Text>
        </FlexWrapper>
      </FlexWrapper>
    </Widget>
  );
};

export default ConnectionDrawerDetails;
