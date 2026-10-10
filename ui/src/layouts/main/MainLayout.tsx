import type { FC, PropsWithChildren } from "react";

import Flex, { AlignItems, FlexDirection, FlexVariant } from "@galaxy-io/dls/layout/Flex";
import { Radius } from "@galaxy-io/dls/theme/enums";

import { MAIN_LAYOUT_GUTTER } from "@/layouts/main/constants";
import MainLayoutNavbar from "@/layouts/main/MainLayoutNavbar";

const MainLayout: FC<PropsWithChildren> = ({ children }) => {
  return (
    <Flex alignItems={AlignItems.START} height="100%" fillWidth direction={FlexDirection.COLUMN}>
      <MainLayoutNavbar />
      <Flex
        direction={FlexDirection.COLUMN}
        grow={1}
        basis={0}
        fillWidth
        minWidth={0}
        minHeight={0}
        padding={MAIN_LAYOUT_GUTTER}
        overflow="hidden"
      >
        <Flex
          direction={FlexDirection.COLUMN}
          grow={1}
          basis={0}
          fillWidth
          minHeight={0}
          variant={FlexVariant.PRIMARY}
          hasBorder
          radius={Radius.LG}
          overflow="hidden"
        >
          {children}
        </Flex>
      </Flex>
    </Flex>
  );
};

export default MainLayout;
