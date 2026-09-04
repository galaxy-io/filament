import { styled } from "@linaria/react";
import { ArrowUpRightIcon } from "@phosphor-icons/react";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
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
        <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.SMALL} fillWidth>
          {AUTH_LAYOUT_ASIDE_LINKS.map(({ icon, label, description, url }) => (
            <Widget
              key={label}
              variant={WidgetVariant.SECONDARY_ALT}
              padding={AUTH_LAYOUT_ASIDE_LINK_PADDING}
              onClick={() => window.open(url, "_blank")}
              fillWidth
            >
              <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM} fillWidth>
                <Icon
                  component={icon}
                  size={16}
                  weight={IconWeight.FILL}
                  variant={IconVariant.SECONDARY_ALT}
                />
                <FlexItem grow={1} minWidth={0}>
                  <FlexWrapper direction={FlexDirection.COLUMN} gap={FlexGap.XXSMALL}>
                    <Text
                      size={TextSize.BODY_MD}
                      weight={TextWeight.MEDIUM}
                      variant={TextVariant.PRIMARY_ALT}
                    >
                      {label}
                    </Text>
                    <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY_ALT}>
                      {description}
                    </Text>
                  </FlexWrapper>
                </FlexItem>
                <Icon component={ArrowUpRightIcon} size={12} variant={IconVariant.TERTIARY_ALT} />
              </FlexWrapper>
            </Widget>
          ))}
        </FlexWrapper>
      </AsideContentWrapper>
    </AuthLayoutAsideField>
  </AsideWrapper>
);

export default AuthLayoutAside;
