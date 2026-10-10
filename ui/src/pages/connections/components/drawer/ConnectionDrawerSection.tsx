import type { ComponentProps, FC, PropsWithChildren } from "react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Box from "@galaxy-io/dls/layout/Box";
import EmptyLayout, { EmptyLayoutSize } from "@galaxy-io/dls/layout/EmptyLayout";
import Widget from "@galaxy-io/dls/widget/Widget";

interface ConnectionDrawerSectionProps {
  header: string;
  icon: ComponentProps<typeof Widget>["icon"];
  count: number;
  emptyHeader: string;
  emptyMessage: string;
  isOpenInitial?: boolean;
  isFlush?: boolean;
}

const ConnectionDrawerSection: FC<PropsWithChildren<ConnectionDrawerSectionProps>> = ({
  header,
  icon,
  count,
  emptyHeader,
  emptyMessage,
  isOpenInitial = false,
  isFlush = false,
  children,
}) => {
  return (
    <Widget
      isCollapsible
      header={header}
      icon={icon}
      actions={
        <Chip
          hasBorder
          isPill
          count={count}
          size={ChipSize.SMALL}
          variant={ChipVariant.SECONDARY}
        />
      }
      defaultIsOpen={isOpenInitial}
      isFlush={count > 0 && isFlush}
    >
      {count === 0 ? (
        <Box padding={12}>
          <EmptyLayout
            size={EmptyLayoutSize.SMALL}
            header={emptyHeader}
            description={emptyMessage}
          />
        </Box>
      ) : (
        children
      )}
    </Widget>
  );
};

export default ConnectionDrawerSection;
