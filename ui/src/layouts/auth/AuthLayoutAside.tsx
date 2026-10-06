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
  AUTH_LAYOUT_ASIDE_LINK_PADDING,
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
        <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={8} fillWidth>
          {AUTH_LAYOUT_ASIDE_LINKS.map(({ icon, label, description, url }) => (
            <Widget
              isInteractive
              key={label}
              /* @dls-migrate widget.WidgetVariant.SECONDARY_ALT: Removed: use the plain step, plus `isSelected` where it meant chosen. */ variant={
                WidgetVariant.SECONDARY_ALT
              }
              /* @dls-migrate widget.padding-other: The body inset is fixed at 12px: remove `padding` (use `isFlush` for 0). */ padding={
                AUTH_LAYOUT_ASIDE_LINK_PADDING
              }
              onClick={() => window.open(url, "_blank")}
              /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */ fillWidth
            >
              <Flex alignItems={AlignItems.CENTER} gap={12} fillWidth>
                <Icon
                  component={icon}
                  size={16}
                  weight={IconWeight.FILL}
                  /* @dls-migrate icon.IconVariant.SECONDARY_ALT: Inverse is a scope, not a member: wrap the content in `<GalaxyProvider isScoped theme={…}>` (the opposite theme) and use `IconVariant.SECONDARY` inside it. */ variant={
                    IconVariant.SECONDARY_ALT
                  }
                />
                <FlexItem grow={1} minWidth={0}>
                  <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={2}>
                    <Text
                      size={TextSize.BODY_MD}
                      weight={TextWeight.MEDIUM}
                      /* @dls-migrate text.TextVariant.PRIMARY_ALT: Inverse is a scope, not a member: wrap the content in `<GalaxyProvider isScoped theme={…}>` (the opposite theme) and use `TextVariant.PRIMARY` inside it. */ variant={
                        TextVariant.PRIMARY_ALT
                      }
                    >
                      {label}
                    </Text>
                    <Text
                      size={
                        TextSize.BODY_SM
                      } /* @dls-migrate text.TextVariant.TERTIARY_ALT: Inverse is a scope, not a member: wrap the content in `<GalaxyProvider isScoped theme={…}>` (the opposite theme) and use `TextVariant.TERTIARY` inside it. */
                      variant={TextVariant.TERTIARY_ALT}
                    >
                      {description}
                    </Text>
                  </Flex>
                </FlexItem>
                <Icon
                  component={ArrowUpRightIcon}
                  size={
                    12
                  } /* @dls-migrate icon.IconVariant.TERTIARY_ALT: Inverse is a scope, not a member: wrap the content in `<GalaxyProvider isScoped theme={…}>` (the opposite theme) and use `IconVariant.TERTIARY` inside it. */
                  variant={IconVariant.TERTIARY_ALT}
                />
              </Flex>
            </Widget>
          ))}
        </Flex>
      </AsideContentWrapper>
    </AuthLayoutAsideField>
  </AsideWrapper>
);

export default AuthLayoutAside;
