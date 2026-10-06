import { styled } from "@linaria/react";
import { Link, useRouteContext } from "@tanstack/react-router";

import GalaxyFilamentWordmark from "@galaxy-io/dls/brand/GalaxyFilamentWordmark";
import GalaxyLogomark from "@galaxy-io/dls/brand/GalaxyLogomark";
import { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import DocsButton from "@/components/DocsButton";
import GithubButton from "@/components/GithubButton";
import ThemeButton from "@/components/ThemeButton";

import MainLayoutSettingsButton from "@/layouts/main/MainLayoutSettingsButton";

import { type TRoutes, useRouteMatch } from "@/hooks/useRouteMatch";

const MAIN_NAVBAR_HEIGHT = 52;
const MAIN_NAVBAR_RAIL_WIDTH = 200;

export interface NavItem {
  to: TRoutes;
  label: string;
}

export const NAV_ITEMS: NavItem[] = [
  { to: "/observability", label: "Observability" },
  { to: "/pipelines", label: "Pipelines" },
  { to: "/sources", label: "Sources" },
  { to: "/sinks", label: "Sinks" },
];

const NavbarWrapper = styled.div`
  width: 100%;
  height: ${MAIN_NAVBAR_HEIGHT}px;

  padding: 0 16px;

  display: flex;
  align-items: center;
  justify-content: space-between;

  background-color: ${t.color.background.base};
`;

const NavTabWrapper = styled.div<{ $isActive?: boolean }>`
  padding-bottom: 6px;

  border-bottom: 2px solid
    ${({ $isActive }) => ($isActive ? t.color.text.primary : "transparent")};

  transition: border-color 100ms ease;
`;

const NavTabsWrapper = styled.div`
  height: 100%;

  padding: 16px 24px 0;

  display: flex;
  gap: 24px;
  align-items: flex-start;
`;

const MainLayoutNavTab = ({ item }: { item: NavItem }) => {
  const { isRouteMatch: isActive } = useRouteMatch({
    route: item.to,
    fuzzy: true,
  });

  return (
    <Link to={item.to}>
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
};

const MainLayoutNavbar = () => {
  const { session } = useRouteContext({ from: "/_app" });

  return (
    <NavbarWrapper>
      <Link to={"/"}>
        <Flex alignItems={AlignItems.CENTER} gap={12} width={MAIN_NAVBAR_RAIL_WIDTH}>
          <FlexItem shrink={0}>
            <GalaxyLogomark size={12} />
          </FlexItem>
          <FlexItem shrink={0}>
            <GalaxyFilamentWordmark size={18} />
          </FlexItem>
          <FlexItem shrink={0}>
            <DocsButton variant={ButtonVariant.TERTIARY} />
          </FlexItem>
        </Flex>
      </Link>
      <NavTabsWrapper>
        {NAV_ITEMS.map((item) => (
          <MainLayoutNavTab key={item.to} item={item} />
        ))}
      </NavTabsWrapper>
      <Flex
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.END}
        gap={8}
        width={MAIN_NAVBAR_RAIL_WIDTH}
      >
        <FlexItem shrink={0}>
          <GithubButton />
        </FlexItem>
        {!session.isAuthenticated && (
          <FlexItem shrink={0}>
            <ThemeButton />
          </FlexItem>
        )}
        <FlexItem shrink={0}>
          <MainLayoutSettingsButton />
        </FlexItem>
      </Flex>
    </NavbarWrapper>
  );
};

export default MainLayoutNavbar;
