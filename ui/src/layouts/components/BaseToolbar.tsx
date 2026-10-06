import { styled } from "@linaria/react";

import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";

const LeadingWrapper = styled.div`
  display: flex;
  flex-direction: column;
  gap: 4px;
  flex: 1;
  min-width: 0;
  overflow: hidden;
`;

const TrailingWrapper = styled.div`
  display: flex;
  align-items: center;
  gap: 8px;
  flex-shrink: 0;
`;

interface BaseToolbarProps {
  leadingActions: React.ReactNode[];
  trailingActions?: React.ReactNode[];
  noPadding?: boolean;
}

const BaseToolbar = ({ leadingActions, trailingActions, noPadding }: BaseToolbarProps) => {
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
      <LeadingWrapper>{leadingActions}</LeadingWrapper>
      {hasTrailingActions && <TrailingWrapper>{trailingActions}</TrailingWrapper>}
    </Flex>
  );
};

export default BaseToolbar;
