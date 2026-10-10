import type { FC } from "react";

import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextWeight } from "@galaxy-io/dls/text/Text";

interface ConnectionDrawerKeyValueRowProps {
  label: string;
  value: React.ReactNode;
}

const ConnectionDrawerKeyValueRow: FC<ConnectionDrawerKeyValueRowProps> = ({ label, value }) => {
  return (
    <Flex
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      gap={8}
      fillWidth
    >
      <Text weight={TextWeight.MEDIUM}>{label}</Text>
      {value}
    </Flex>
  );
};

export default ConnectionDrawerKeyValueRow;
