import { styled } from "@linaria/react";
import { ArrowUpRightIcon } from "@phosphor-icons/react";

import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
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

const AsideWrapper = styled.div`
  flex: 1;
  min-width: 0;

  display: flex;

  @media (max-width: ${AUTH_LAYOUT_ASIDE_BREAKPOINT}px) {
    display: none;
  }
`;

const AsideContentWrapper = styled.div`
  width: ${AUTH_LAYOUT_ASIDE_CONTENT_WIDTH}px;
`;

const AuthLayoutAside = () => (
  <AsideWrapper>
    <AuthLayoutAsideField>
      <AsideContentWrapper>
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
      </AsideContentWrapper>
    </AuthLayoutAsideField>
  </AsideWrapper>
);

export default AuthLayoutAside;
