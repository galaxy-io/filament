import type { FC } from "react";

import { ArrowUpRightIcon } from "@phosphor-icons/react";

import { useMediaQuery } from "@galaxy-io/dls/hooks/useMediaQuery";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget, { WidgetVariant } from "@galaxy-io/dls/widget/Widget";

import AuthLayoutAsideField from "@/layouts/auth/AuthLayoutAsideField";
import {
  AUTH_LAYOUT_ASIDE_BREAKPOINT,
  AUTH_LAYOUT_ASIDE_CONTENT_WIDTH,
  AUTH_LAYOUT_ASIDE_LINKS,
} from "@/layouts/auth/constants";

const AuthLayoutAside: FC = () => {
  const isNarrow = useMediaQuery(`(max-width: ${AUTH_LAYOUT_ASIDE_BREAKPOINT}px)`);

  if (isNarrow) {
    return null;
  }

  return (
    <Flex grow={1} basis={0} minWidth={0}>
      <AuthLayoutAsideField>
        <Box width={AUTH_LAYOUT_ASIDE_CONTENT_WIDTH}>
          <Flex alignItems={AlignItems.STRETCH} direction={FlexDirection.COLUMN} gap={8} fillWidth>
            {AUTH_LAYOUT_ASIDE_LINKS.map(({ icon, label, description, url }) => (
              <Widget
                isInteractive
                key={label}
                variant={WidgetVariant.SECONDARY}
                isFlush
                onClick={() => window.open(url, "_blank")}
              >
                <Flex alignItems={AlignItems.CENTER} gap={12} padding={[12, 16]} fillWidth>
                  <Icon
                    component={icon}
                    size={16}
                    weight={IconWeight.FILL}
                    variant={IconVariant.SECONDARY}
                  />
                  <FlexItem grow={1} minWidth={0}>
                    <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={2}>
                      <Text
                        size={TextSize.BODY_MD}
                        weight={TextWeight.MEDIUM}
                        variant={TextVariant.PRIMARY}
                      >
                        {label}
                      </Text>
                      <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                        {description}
                      </Text>
                    </Flex>
                  </FlexItem>
                  <Icon component={ArrowUpRightIcon} size={12} variant={IconVariant.TERTIARY} />
                </Flex>
              </Widget>
            ))}
          </Flex>
        </Box>
      </AuthLayoutAsideField>
    </Flex>
  );
};

export default AuthLayoutAside;
