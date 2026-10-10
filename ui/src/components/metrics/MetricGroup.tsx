import type { FC, ReactNode } from "react";

import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Widget from "@galaxy-io/dls/widget/Widget";

interface MetricGroupProps {
  primary?: ReactNode;
  children: ReactNode;
}

const MetricGroup: FC<MetricGroupProps> = ({ primary, children }) => (
  <Box maxWidth="100%">
    <Widget isFlush>
      <Flex alignItems={AlignItems.CENTER} gap={16} padding={[12, 0, 12, 12]}>
        {primary && (
          <FlexItem grow={1} shrink={0}>
            {primary}
          </FlexItem>
        )}
        <Flex
          alignItems={AlignItems.CENTER}
          gap={12}
          padding={[0, 12, 0, 0]}
          minWidth={0}
          overflow="auto"
        >
          {children}
        </Flex>
      </Flex>
    </Widget>
  </Box>
);

export default MetricGroup;
