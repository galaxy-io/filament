import { GithubLogoIcon } from "@phosphor-icons/react";
import { styled } from "@linaria/react";
import { Link } from "@tanstack/react-router";

import FlexWrapper, { AlignItems, FlexGap, JustifyContent } from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyLogomark from "@galaxy-io/dls/icons/GalaxyLogomark";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { useGalaxyTheme, withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import { GalaxyTheme } from "@galaxy-io/dls/theme/constants";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { NAV_ITEMS } from "@/layouts/main/constants";

import { APP_VERSION, GITHUB_REPO_URL } from "@/constants";

const NAVBAR_HEIGHT = 52;

const NavbarWrapper = withTheme(styled.div<PropsWithTheme>`
  width: 100%;
  height: ${NAVBAR_HEIGHT}px;

  padding: 0 16px;

  display: flex;
  align-items: center;
  justify-content: space-between;

  background-color: ${({ theme }) => theme.color.background.base};
`);

const NavTabWrapper = withTheme(styled.div<PropsWithTheme<{ $isActive?: boolean }>>`
  padding-bottom: 8px;

  border-bottom: 2px solid
    ${({ theme, $isActive }) => ($isActive ? theme.color.text.primary : "transparent")};

  transition: border-color 100ms ease;
`);

const NavTabsWrapper = styled.div`
  height: 100%;

  padding: 16px 24px 0;

  display: flex;
  gap: 24px;
  align-items: flex-start;
`;

const MainLayoutNavbar = () => {
  const { galaxyTheme } = useGalaxyTheme();

  return (
    <NavbarWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <GalaxyLogomark height={12} isDark={galaxyTheme === GalaxyTheme.DARK} />
        <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM} isMonospace>
          FILAMENT
        </Text>
      </FlexWrapper>
      <NavTabsWrapper>
        {NAV_ITEMS.map((item) => (
          <Link key={item.to} to={item.to}>
            {({ isActive }: { isActive: boolean }) => (
              <NavTabWrapper $isActive={isActive}>
                <Text
                  size={TextSize.BODY_MD}
                  variant={isActive ? TextVariant.PRIMARY : TextVariant.SECONDARY}
                  weight={isActive ? TextWeight.MEDIUM : TextWeight.REGULAR}
                  cursor="pointer"
                >
                  {item.label}
                </Text>
              </NavTabWrapper>
            )}
          </Link>
        ))}
      </NavTabsWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} justifyContent={JustifyContent.END} gap={FlexGap.SMALL}>
        <a href={GITHUB_REPO_URL} target="_blank" rel="noreferrer">
          <Icon component={GithubLogoIcon} size={16} variant={IconVariant.SECONDARY} />
        </a>
        <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY_ALT} isMonospace>
          {APP_VERSION}
        </Text>
      </FlexWrapper>
    </NavbarWrapper>
  );
};

export default MainLayoutNavbar;
