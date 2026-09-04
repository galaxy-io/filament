import type { PropsWithChildren, ReactNode } from "react";

import FlexWrapper, { AlignItems, FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";

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
  <FlexWrapper
    direction={FlexDirection.COLUMN}
    alignItems={AlignItems.STRETCH}
    overflow="hidden"
    fillWidth
    fillHeight
  >
    <FlexWrapper padding="16px" fillWidth>
      <BaseHeader title={title} size={BaseHeaderSize.LARGE} actions={actions} />
    </FlexWrapper>
    <HorizontalDivider />
    {children}
  </FlexWrapper>
);

export default SettingsPanelLayout;
