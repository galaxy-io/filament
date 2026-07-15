import FlexWrapper, { AlignItems, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";

interface BaseToolbarProps {
  leadingActions: React.ReactNode[];
  trailingActions?: React.ReactNode[];
}

const BaseToolbar = ({ leadingActions, trailingActions }: BaseToolbarProps) => {
  const hasTrailingActions = trailingActions && trailingActions.length > 0;

  return (
    <FlexWrapper
      alignItems={AlignItems.CENTER}
      justifyContent={JustifyContent.SPACE_BETWEEN}
      overflow="scroll"
      gap={6}
      fillWidth
    >
      <FlexWrapper alignItems={AlignItems.CENTER} gap={6} fillWidth={!hasTrailingActions}>
        {leadingActions}
      </FlexWrapper>
      {hasTrailingActions && (
        <FlexWrapper alignItems={AlignItems.CENTER} gap={6}>
          {trailingActions}
        </FlexWrapper>
      )}
    </FlexWrapper>
  );
};

export default BaseToolbar;
