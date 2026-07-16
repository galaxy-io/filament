import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import MainLayoutNavbar from "@/layouts/main/MainLayoutNavbar";

const MainLayoutBodyWrapper = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  width: 100%;
  min-width: 0;
  min-height: 0;

  padding: 12px;

  display: flex;
  flex-direction: column;
  overflow: hidden;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const MainLayoutIslandWrapper = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  width: 100%;
  min-height: 0;

  display: flex;
  flex-direction: column;

  background-color: ${({ theme }) => theme.color.background.primary};

  border: 0.5px solid ${({ theme }) => theme.color.border.primary};
  border-radius: 6px;

  overflow: hidden;
`);

const MainLayout = ({ children }: PropsWithChildren) => {
  return (
    <FlexWrapper fillHeight fillWidth direction={FlexDirection.COLUMN}>
      <MainLayoutNavbar />
      <FlexItem grow={0} shrink={0} fillWidth>
        <HorizontalDivider />
      </FlexItem>
      <MainLayoutBodyWrapper>
        <MainLayoutIslandWrapper>{children}</MainLayoutIslandWrapper>
      </MainLayoutBodyWrapper>
    </FlexWrapper>
  );
};

export default MainLayout;
