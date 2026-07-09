import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

interface PageLayoutProps {
  title: string;
  isLoading?: boolean;
  isError?: boolean;
  isEmpty?: boolean;
  emptyMessage?: string;
}

const PageLayoutWrapper = styled.div`
  height: 100%;
  width: 100%;
  min-height: 0;

  padding: 16px;

  display: flex;
  flex-direction: column;
  gap: 16px;

  overflow-y: auto;
`;

const PageLayout = ({
  title,
  isLoading,
  isError,
  isEmpty,
  emptyMessage = "Nothing here yet.",
  children,
}: PropsWithChildren<PageLayoutProps>) => {
  return (
    <PageLayoutWrapper>
      <Text size={TextSize.HEADING_SM} weight={TextWeight.MEDIUM}>
        {title}
      </Text>
      {isLoading ? (
        <FlexWrapper
          fillWidth
          fillHeight
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.CENTER}
        >
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            Loading...
          </Text>
        </FlexWrapper>
      ) : isError ? (
        <FlexWrapper
          fillWidth
          fillHeight
          direction={FlexDirection.COLUMN}
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.CENTER}
          gap={FlexGap.XSMALL}
        >
          <Text size={TextSize.BODY_MD} variant={TextVariant.ERROR}>
            Failed to load
          </Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            Is the Filament server running?
          </Text>
        </FlexWrapper>
      ) : isEmpty ? (
        <FlexWrapper
          fillWidth
          fillHeight
          alignItems={AlignItems.CENTER}
          justifyContent={JustifyContent.CENTER}
        >
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            {emptyMessage}
          </Text>
        </FlexWrapper>
      ) : (
        children
      )}
    </PageLayoutWrapper>
  );
};

export default PageLayout;
