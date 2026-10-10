import type { FC } from "react";

import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";

interface BaseToolbarProps {
  leadingActions: React.ReactNode[];
  trailingActions?: React.ReactNode[];
  noPadding?: boolean;
}

const BaseToolbar: FC<BaseToolbarProps> = ({ leadingActions, trailingActions, noPadding }) => {
  const hasTrailingActions = trailingActions && trailingActions.length > 0;

  return (
    <Flex
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      overflow="scroll"
      gap={8}
      fillWidth
      padding={noPadding ? 0 : [8, 12]}
    >
      <Flex
        direction={FlexDirection.COLUMN}
        gap={4}
        grow={1}
        basis={0}
        minWidth={0}
        overflow="hidden"
      >
        {leadingActions}
      </Flex>
      {hasTrailingActions && (
        <Flex alignItems={AlignItems.CENTER} gap={8} shrink={0}>
          {trailingActions}
        </Flex>
      )}
    </Flex>
  );
};

export default BaseToolbar;
