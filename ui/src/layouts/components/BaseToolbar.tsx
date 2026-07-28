import { styled } from "@linaria/react";

import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";

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
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      overflow="scroll"
      gap={8}
      fillWidth
      padding={noPadding ? "0" : "8px 12px"}
    >
      <LeadingWrapper>{leadingActions}</LeadingWrapper>
      {hasTrailingActions && <TrailingWrapper>{trailingActions}</TrailingWrapper>}
    </FlexWrapper>
  );
};

export default BaseToolbar;
