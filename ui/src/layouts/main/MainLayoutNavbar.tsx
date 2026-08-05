import { styled } from "@linaria/react";
import { Link } from "@tanstack/react-router";

import FlexItem from "@galaxy-io/dls/containers/FlexItem";
import FlexWrapper, {
  AlignItems,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import GalaxyLogomark from "@galaxy-io/dls/icons/GalaxyLogomark";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";

import DocsButton from "@/components/DocsButton";
import FilamentWordmark from "@/components/FilamentWordmark";
import GithubButton from "@/components/GithubButton";

import { type TRoutes, useRouteMatch } from "@/hooks/useRouteMatch";

export const NAVBAR_HEIGHT = 52;

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
  return (
    <NavbarWrapper>
      <Link to={"/"}>
        <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.MEDIUM} width={200}>
          <FlexItem shrink={0}>
            <GalaxyLogomark height={12} />
          </FlexItem>
          <FlexItem shrink={0}>
            <FilamentWordmark height={18} />
          </FlexItem>
        </FlexWrapper>
      </Link>
      <NavTabsWrapper>
        {NAV_ITEMS.map((item) => (
          <MainLayoutNavTab key={item.to} item={item} />
        ))}
      </NavTabsWrapper>
      <FlexWrapper
        alignItems={AlignItems.CENTER}
        justifyContent={JustifyContent.END}
        gap={FlexGap.LARGE}
        width={200}
      >
        <DocsButton path="/" />
        <GithubButton />
      </FlexWrapper>
    </NavbarWrapper>
  );
};

export default MainLayoutNavbar;
