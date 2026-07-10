import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import Bold from "@galaxy-io/dls/text/Bold";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import type { ProviderSpec } from "@/gen/ingestion/v1/providers_pb";

interface ProviderDrawerDetailsProps {
  provider: ProviderSpec;
}

const ProviderDrawerDetails = ({ provider }: ProviderDrawerDetailsProps) => {
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
            Provider ID
          </Text>
          <Text size={TextSize.BODY_SM}>{provider.name}</Text>
        </FlexWrapper>
        <FlexWrapper
          fillWidth
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.SPACE_BETWEEN}
        >
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            Display Name
          </Text>
          <Text size={TextSize.BODY_SM}>{provider.displayName || "-"}</Text>
        </FlexWrapper>
        <HorizontalDivider />
        <FlexWrapper fillWidth alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
          <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY}>
            Created on <Bold>Jan 1, 2025</Bold>
          </Text>
        </FlexWrapper>
      </FlexWrapper>
    </Widget>
  );
};

export default ProviderDrawerDetails;
