import type { PropsWithChildren } from "react";

import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

interface SettingsNavigationGroupProps {
  title: string;
}

const SettingsNavigationGroup = ({
  title,
  children,
}: PropsWithChildren<SettingsNavigationGroupProps>) => (
  <FlexWrapper direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} fillWidth>
    <FlexWrapper padding="0 8px 8px">
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        {title}
      </Text>
    </FlexWrapper>
    <FlexWrapper direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} gap={4} fillWidth>
      {children}
    </FlexWrapper>
  </FlexWrapper>
);

export default SettingsNavigationGroup;
