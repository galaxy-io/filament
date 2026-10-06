import type { ReactNode } from "react";

import type { Icon as PhosphorIcon } from "@phosphor-icons/react";

import EmptyState from "@galaxy-io/dls/feedback/EmptyState";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";

import { LAYOUT_SIZE_TO_EMPTY_STATE_SIZE_MAP } from "@/layouts/constants";
import { LayoutSize } from "@/layouts/types";

interface EmptyLayoutProps {
  size?: LayoutSize;
  icon?: PhosphorIcon;
  graphic?: ReactNode;
  header: string;
  description?: string;
  actions?: ReactNode;
}

const EmptyLayout = ({
  size = LayoutSize.MEDIUM,
  icon,
  graphic,
  header,
  description,
  actions,
}: EmptyLayoutProps) => {
  const mark = graphic === undefined ? { icon } : { graphic };

  return (
    <Flex
      fillWidth
      height="100%"
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.CENTER}
    >
      <EmptyState
        {...mark}
        size={LAYOUT_SIZE_TO_EMPTY_STATE_SIZE_MAP[size]}
        header={header}
        description={description}
        actions={actions}
      />
    </Flex>
  );
};

export default EmptyLayout;
