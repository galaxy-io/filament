import type { PropsWithChildren } from "react";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

interface SettingsNavigationGroupProps {
  title: string;
}

const SettingsNavigationGroup = ({
  title,
  children,
}: PropsWithChildren<SettingsNavigationGroupProps>) => (
  <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} fillWidth>
    <Flex alignItems={AlignItems.START} padding={[0, 8, 8]}>
      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
        {title}
      </Text>
    </Flex>
    <Flex direction={FlexDirection.COLUMN} alignItems={AlignItems.STRETCH} gap={4} fillWidth>
      {children}
    </Flex>
  </Flex>
);

export default SettingsNavigationGroup;
