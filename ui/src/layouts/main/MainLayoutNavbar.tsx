import { styled } from "@linaria/react";
import { Link } from "@tanstack/react-router";
import GitHubButton from "react-github-btn";

import FlexWrapper, { AlignItems, FlexGap } from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyLogomark from "@galaxy-io/dls/icons/GalaxyLogomark";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import { NAV_ITEMS, NAVBAR_HEIGHT } from "@/layouts/main/constants";

import { useRouteMatch } from "@/hooks/useRouteMatch";

import { GITHUB_REPO_URL } from "@/constants";

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
  const { isRouteMatch: isPipelinesActive } = useRouteMatch({
    route: "/pipelines",
    fuzzy: true,
  });
  const { isRouteMatch: isConnectorsActive } = useRouteMatch({
    route: "/connectors",
    fuzzy: true,
  });

  const isActiveByRoute: Record<string, boolean> = {
    "/pipelines": isPipelinesActive,
    "/connectors": isConnectorsActive,
  };

  return (
    <NavbarWrapper>
      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM}>
        <GalaxyLogomark height={12} />
        <Text isMonospace>FILAMENT</Text>
      </FlexWrapper>
      <NavTabsWrapper>
        {NAV_ITEMS.map((item) => {
          const isActive = isActiveByRoute[item.to] ?? false;
          return (
            <Link key={item.to} to={item.to}>
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
            </Link>
          );
        })}
      </NavTabsWrapper>
      <GitHubButton
        href={GITHUB_REPO_URL}
        data-color-scheme="no-preference: dark; light: light; dark: dark;"
        data-show-count="true"
        aria-label="Star galaxy-io/filament on GitHub"
      >
        Star
      </GitHubButton>
    </NavbarWrapper>
  );
};

export default MainLayoutNavbar;
