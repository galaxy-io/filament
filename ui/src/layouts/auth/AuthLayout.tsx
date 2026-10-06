import type { PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import GalaxyFilamentWordmark from "@galaxy-io/dls/brand/GalaxyFilamentWordmark";
import GalaxyLogomark from "@galaxy-io/dls/brand/GalaxyLogomark";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Link from "@galaxy-io/dls/links/Link";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import AuthLayoutAside from "@/layouts/auth/AuthLayoutAside";
import {
  AUTH_LAYOUT_CONTENT_WIDTH,
  AUTH_LAYOUT_INSET,
  SUPPORT_EMAIL,
} from "@/layouts/auth/constants";

const LayoutWrapper = styled.div`
  flex: 1;
  min-height: 0;

  display: flex;

  background-color: ${t.color.background.base};
`;

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
      <Flex alignItems={AlignItems.CENTER} gap={8}>
        <GalaxyLogomark size={12} />
        <GalaxyFilamentWordmark size={18} />
      </Flex>
      <ContentWrapper>
        <Content>{children}</Content>
      </ContentWrapper>
      <Flex alignItems={AlignItems.CENTER}>
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
          Need help? Email us at <Link href={`mailto:${SUPPORT_EMAIL}`}>{SUPPORT_EMAIL}</Link>
        </Text>
      </Flex>
    </MainWrapper>
    <AuthLayoutAside />
  </LayoutWrapper>
);

export default AuthLayout;
