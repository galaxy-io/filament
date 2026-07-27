import type { ComponentProps, PropsWithChildren } from "react";

import Accordion from "@galaxy-io/dls/accordion/Accordion";
import Badge, { BadgeSize, BadgeVariant } from "@galaxy-io/dls/badge/Badge";

import EmptyLayout, { EmptyLayoutSize } from "@/layouts/EmptyLayout";

interface ConnectionDrawerSectionProps {
  header: string;
  icon: ComponentProps<typeof Accordion>["icon"];
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
    <Accordion
      header={header}
      icon={icon}
      metric={<Badge count={count} size={BadgeSize.SMALL} variant={BadgeVariant.SECONDARY} />}
      isOpenInitial={isOpenInitial}
      padding={count > 0 ? 0 : "24px"}
    >
      {count === 0 ? (
        <EmptyLayout size={EmptyLayoutSize.SMALL} header={emptyHeader} message={emptyMessage} />
      ) : (
        children
      )}
    </Accordion>
  );
};

export default ConnectionDrawerSection;
