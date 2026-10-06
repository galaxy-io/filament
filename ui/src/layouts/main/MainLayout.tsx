import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import { HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { MAIN_LAYOUT_GUTTER } from "@/layouts/main/constants";
import MainLayoutNavbar from "@/layouts/main/MainLayoutNavbar";

const MainLayoutBodyWrapper = styled.div`
  flex: 1;
  width: 100%;
  min-width: 0;
  min-height: 0;

  padding: ${MAIN_LAYOUT_GUTTER}px;

  display: flex;
  flex-direction: column;
  overflow: hidden;

  background-color: ${t.color.background.base};
`;

const MainLayoutIslandWrapper = styled.div`
  flex: 1;
  width: 100%;
  min-height: 0;

  display: flex;
  flex-direction: column;

  background-color: ${t.color.background.primary};

  border: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  border-radius: ${t.radius.lg};

  overflow: hidden;
`;

const MainLayout = ({ children }: PropsWithChildren) => {
  return (
    <Flex alignItems={AlignItems.START} height="100%" fillWidth direction={FlexDirection.COLUMN}>
      <MainLayoutNavbar />
      <MainLayoutBodyWrapper>
        <MainLayoutIslandWrapper>{children}</MainLayoutIslandWrapper>
      </MainLayoutBodyWrapper>
    </Flex>
  );
};

export default MainLayout;
