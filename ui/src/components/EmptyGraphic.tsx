import type { FC, PropsWithChildren } from "react";

import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";

const EMPTY_GRAPHIC_WIDTH = 560;
const EMPTY_GRAPHIC_HEIGHT = 200;

const EmptyGraphic: FC<PropsWithChildren> = ({ children }) => (
  <Flex
    width={EMPTY_GRAPHIC_WIDTH}
    height={EMPTY_GRAPHIC_HEIGHT}
    alignItems={AlignItems.CENTER}
    justifyContent={JustifyContent.CENTER}
  >
    {children}
  </Flex>
);

export default EmptyGraphic;
