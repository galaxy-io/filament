import type { FC, ReactNode } from "react";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";

interface ObservabilityMetricsValueProps {
  value: string;
  mark: ReactNode;
}

const ObservabilityMetricsValue: FC<ObservabilityMetricsValueProps> = ({ value, mark }) => (
  <Flex alignItems={AlignItems.CENTER} gap={8} minWidth={0}>
    <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM} isTabular lineClamp={1}>
      {value}
    </Text>
    {mark}
  </Flex>
);

export default ObservabilityMetricsValue;
