import type { PropsWithChildren, ReactNode } from "react";

import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";

import BaseHeader, { BaseHeaderSize } from "@/layouts/components/BaseHeader";

interface SettingsPanelLayoutProps {
  title: string;
  actions?: ReactNode[];
}

const SettingsPanelLayout = ({
  title,
  actions,
  children,
}: PropsWithChildren<SettingsPanelLayoutProps>) => (
  <Flex
    direction={FlexDirection.COLUMN}
    alignItems={AlignItems.STRETCH}
    overflow="hidden"
    fillWidth
    height="100%"
  >
    <Flex alignItems={AlignItems.START} padding={16} fillWidth>
      <BaseHeader title={title} size={BaseHeaderSize.LARGE} actions={actions} />
    </Flex>
    <Divider />
    {children}
  </Flex>
);

export default SettingsPanelLayout;
