import type { FC } from "react";

import type { LinkProps } from "@tanstack/react-router";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Link, { LinkUnderline } from "@galaxy-io/dls/links/Link";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";

import RouterLink from "@/components/RouterLink";

interface AuthFormFooterProps {
  prompt: string;
  to: LinkProps["to"];
  label: string;
}

const AuthFormFooter: FC<AuthFormFooterProps> = ({ prompt, to, label }) => (
  <Flex alignItems={AlignItems.CENTER} gap={8} fillWidth>
    <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
      {prompt}
    </Text>
    <Text size={TextSize.BODY_SM}>
      <Link href={to} as={RouterLink} underline={LinkUnderline.HOVER}>
        {label}
      </Link>
    </Text>
  </Flex>
);

export default AuthFormFooter;
