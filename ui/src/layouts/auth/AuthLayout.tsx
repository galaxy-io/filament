import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyFilamentWordmark from "@galaxy-io/dls/icons/GalaxyFilamentWordmark";
import GalaxyLogomark from "@galaxy-io/dls/icons/GalaxyLogomark";
import Bold from "@galaxy-io/dls/text/Bold";
import Selectable from "@galaxy-io/dls/text/Selectable";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import AuthLayoutAside from "@/layouts/auth/AuthLayoutAside";
import { AUTH_LAYOUT_CONTENT_WIDTH, AUTH_LAYOUT_INSET } from "@/layouts/auth/constants";

const LayoutWrapper = withTheme(styled.div<PropsWithTheme>`
  flex: 1;
  min-height: 0;

  display: flex;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const MainWrapper = styled.div`
  flex: 1;
  min-width: 0;

  display: flex;
  flex-direction: column;
  gap: 24px;

  padding: ${AUTH_LAYOUT_INSET}px;
`;

const ContentWrapper = styled.div`
  flex: 1;
  min-height: 0;

  display: flex;
  align-items: center;
  align-items: safe center;
  justify-content: center;

  overflow-y: auto;
`;

const Content = styled.div`
  width: ${AUTH_LAYOUT_CONTENT_WIDTH}px;
`;

const AuthLayout = ({ children }: PropsWithChildren) => (
  <LayoutWrapper>
    <MainWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
        <GalaxyLogomark height={12} />
        <GalaxyFilamentWordmark height={18} />
      </FlexWrapper>
      <ContentWrapper>
        <Content>{children}</Content>
      </ContentWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER}>
        <Text size={TextSize.CAPTION} variant={TextVariant.TERTIARY}>
          Need help? Email us at{" "}
          <Selectable>
            <Bold>support@getgalaxy.io</Bold>
          </Selectable>
        </Text>
      </FlexWrapper>
    </MainWrapper>
    <AuthLayoutAside />
  </LayoutWrapper>
);

export default AuthLayout;
