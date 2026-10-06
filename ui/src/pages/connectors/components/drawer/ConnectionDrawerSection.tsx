import type { ComponentProps, PropsWithChildren } from "react";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";
import Widget from "@galaxy-io/dls/widget/Widget";

import EmptyLayout from "@/layouts/EmptyLayout";
import { LayoutSize } from "@/layouts/types";

interface ConnectionDrawerSectionProps {
  header: string;
  icon: ComponentProps<typeof Widget>["icon"];
  count: number;
  emptyHeader: string;
  emptyMessage: string;
  isOpenInitial?: boolean;
}

const ConnectionDrawerSection = ({
  header,
  icon,
  count,
  emptyHeader,
  emptyMessage,
  isOpenInitial = false,
  children,
}: PropsWithChildren<ConnectionDrawerSectionProps>) => {
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
      /* @dls-migrate accordion.padding-other: The body inset follows `size`: remove `padding` (use `isFlush` for 0). */ padding={
        count > 0 ? 0 : "24px"
      }
    >
      {count === 0 ? (
        <EmptyLayout size={LayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
      ) : (
        children
      )}
    </Widget>
  );
};

export default ConnectionDrawerSection;
