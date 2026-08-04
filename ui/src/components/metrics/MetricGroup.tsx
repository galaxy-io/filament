import type { ReactNode } from "react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import Widget from "@galaxy-io/dls/widget/Widget";
import Spacing from "@galaxy-io/dls/containers/Spacing";

interface MetricGroupProps {
  primary?: ReactNode;
  fillWidth?: boolean;
  children: ReactNode;
}

const MetricGroup = ({ primary, fillWidth, children }: MetricGroupProps) => (
  <Widget fillWidth={fillWidth} maxWidth="100%" padding="12px 0 12px 12px">
    <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.LARGE} fillWidth={fillWidth}>
      {primary && (
        <FlexItem grow={1} shrink={0}>
          {primary}
        </FlexItem>
      )}
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM} minWidth={0} overflow="auto">
        {children}
        <Spacing />
      </FlexWrapper>
    </FlexWrapper>
  </Widget>
);

export default MetricGroup;
