import type { ReactNode } from "react";

// @dls-migrate box.Spacing: Margins are gone: move the space to the parent's `gap` / `padding` and delete the `Spacing`.
import Spacing from "@galaxy-io/dls/containers/Spacing";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Widget from "@galaxy-io/dls/widget/Widget";

interface MetricGroupProps {
  primary?: ReactNode;
  fillWidth?: boolean;
  children: ReactNode;
}

const MetricGroup = ({ primary, fillWidth, children }: MetricGroupProps) => (
  <Box maxWidth="100%">
    <Widget /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */
      fillWidth={
        fillWidth
      } /* @dls-migrate widget.padding-other: The body inset is fixed at 12px: remove `padding` (use `isFlush` for 0). */
      padding="12px 0 12px 12px"
    >
      <Flex alignItems={AlignItems.CENTER} gap={16} fillWidth={fillWidth}>
        {primary && (
          <FlexItem grow={1} shrink={0}>
            {primary}
          </FlexItem>
        )}
        <Flex alignItems={AlignItems.CENTER} gap={12} minWidth={0} overflow="auto">
          {children}
          <Spacing /* @dls-migrate box.Spacing: Margins are gone: move the space to the parent's `gap` / `padding` and delete the `Spacing`. */
          />
        </Flex>
      </Flex>
    </Widget>
  </Box>
);

export default MetricGroup;
