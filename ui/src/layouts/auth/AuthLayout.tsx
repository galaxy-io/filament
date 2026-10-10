import type { FC, PropsWithChildren } from "react";

import { styled } from "@linaria/react";

import GalaxyFilamentWordmark from "@galaxy-io/dls/brand/GalaxyFilamentWordmark";
import GalaxyLogomark from "@galaxy-io/dls/brand/GalaxyLogomark";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Link from "@galaxy-io/dls/links/Link";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import AuthLayoutAside from "@/layouts/auth/AuthLayoutAside";
import {
  AUTH_LAYOUT_CONTENT_WIDTH,
  AUTH_LAYOUT_INSET,
  SUPPORT_EMAIL,
} from "@/layouts/auth/constants";

const ContentWrapper = styled.div`
  flex: 1;
  min-height: 0;

  display: flex;
  align-items: center;
  align-items: safe center;
  justify-content: center;

  overflow-y: auto;
`;

const AuthLayout: FC<PropsWithChildren> = ({ children }) => (
  <Flex grow={1} basis={0} minHeight={0}>
    <Flex
      direction={FlexDirection.COLUMN}
      grow={1}
      basis={0}
      minWidth={0}
      gap={24}
      padding={AUTH_LAYOUT_INSET}
    >
      <Flex alignItems={AlignItems.CENTER} gap={8}>
        <GalaxyLogomark size={12} />
        <GalaxyFilamentWordmark size={18} />
      </Flex>
      <ContentWrapper>
        <Box width={AUTH_LAYOUT_CONTENT_WIDTH}>{children}</Box>
      </ContentWrapper>
      <Flex alignItems={AlignItems.CENTER}>
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
          Need help? Email us at <Link href={`mailto:${SUPPORT_EMAIL}`}>{SUPPORT_EMAIL}</Link>
        </Text>
      </Flex>
    </Flex>
    <AuthLayoutAside />
  </Flex>
);

export default AuthLayout;
