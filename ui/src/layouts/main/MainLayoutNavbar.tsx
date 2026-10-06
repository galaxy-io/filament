import { type AnchorHTMLAttributes, forwardRef, type ReactNode } from "react";

import { Link, useMatchRoute, useRouteContext } from "@tanstack/react-router";

import GalaxyFilamentWordmark from "@galaxy-io/dls/brand/GalaxyFilamentWordmark";
import GalaxyLogomark from "@galaxy-io/dls/brand/GalaxyLogomark";
import { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Tabs, { type TabLinkItem, TabsSize } from "@galaxy-io/dls/navigation/Tabs";
import ThemeSwitcher, { ThemeSwitcherSize } from "@galaxy-io/dls/theme/ThemeSwitcher";

import DocsButton from "@/components/DocsButton";
import GithubButton from "@/components/GithubButton";

import MainLayoutSettingsButton from "@/layouts/main/MainLayoutSettingsButton";

import type { TRoutes } from "@/hooks/useRouteMatch";

const MAIN_NAVBAR_HEIGHT = 48;

const MAIN_NAVBAR_ITEMS: TabLinkItem<TRoutes>[] = [
  { id: "/observability", label: "Observability", href: "/observability" },
  { id: "/pipelines", label: "Pipelines", href: "/pipelines" },
  { id: "/sources", label: "Sources", href: "/sources" },
  { id: "/sinks", label: "Sinks", href: "/sinks" },
];

const MainLayoutNavbarLink = forwardRef<HTMLAnchorElement, AnchorHTMLAttributes<HTMLAnchorElement>>(
  ({ href, ...rest }, ref) => <Link ref={ref} to={href as TRoutes} {...rest} />,
);

const MainLayoutNavbarRail = ({
  children,
  isEnd = false,
}: {
  children: ReactNode;
  isEnd?: boolean;
}) => (
  <Flex
    grow={1}
    basis={0}
    minWidth={0}
    alignItems={AlignItems.CENTER}
    justifyContent={isEnd ? JustifyContent.END : JustifyContent.START}
    gap={isEnd ? 8 : 12}
    padding={[0, 16]}
  >
    {children}
  </Flex>
);

const MainLayoutNavbar = () => {
  const { session } = useRouteContext({ from: "/_app" });
  const matchRoute = useMatchRoute();

  const activeItem = MAIN_NAVBAR_ITEMS.find(
    (item) => matchRoute({ to: item.id, fuzzy: true }) !== false,
  );

  return (
    <Box position="relative" height={MAIN_NAVBAR_HEIGHT} fillWidth>
      <Box position="absolute" inset={{ right: 0, bottom: 0, left: 0 }}>
        <Divider />
      </Box>
      <Flex height="100%">
        <MainLayoutNavbarRail>
          <Link to="/">
            <Flex alignItems={AlignItems.CENTER} gap={12}>
              <GalaxyLogomark size={12} />
              <GalaxyFilamentWordmark size={18} />
            </Flex>
          </Link>
          <DocsButton variant={ButtonVariant.TERTIARY} />
        </MainLayoutNavbarRail>
        <Tabs
          ariaLabel="Main"
          size={TabsSize.MEDIUM}
          items={MAIN_NAVBAR_ITEMS}
          value={activeItem?.id ?? null}
          as={MainLayoutNavbarLink}
        />
        <MainLayoutNavbarRail isEnd>
          <GithubButton />
          {!session.isAuthenticated && <ThemeSwitcher size={ThemeSwitcherSize.SMALL} isIconOnly />}
          <MainLayoutSettingsButton />
        </MainLayoutNavbarRail>
      </Flex>
    </Box>
  );
};

export default MainLayoutNavbar;
