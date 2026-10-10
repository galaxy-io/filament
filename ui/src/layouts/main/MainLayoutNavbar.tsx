import type { FC, ReactNode } from "react";

import { useRouteContext } from "@tanstack/react-router";

import GalaxyFilamentWordmark from "@galaxy-io/dls/brand/GalaxyFilamentWordmark";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import { LinkUnderline } from "@galaxy-io/dls/links/Link";
import Tabs, { type TabLinkItem, TabsSize } from "@galaxy-io/dls/navigation/Tabs";
import { TextSize } from "@galaxy-io/dls/text/Text";
import ThemeSwitcher, { ThemeSwitcherSize } from "@galaxy-io/dls/theme/ThemeSwitcher";

import DocsLink from "@/components/DocsLink";
import GithubButton from "@/components/GithubButton";
import RouterLink from "@/components/RouterLink";

import { MAIN_LAYOUT_GUTTER } from "@/layouts/main/constants";
import MainLayoutSettingsButton from "@/layouts/main/MainLayoutSettingsButton";

import { useFilamentMatchRoute } from "@/module/hooks";
import { createFilamentHref, FilamentPath } from "@/module/paths";

const MAIN_NAVBAR_HEIGHT = 48;

const MAIN_NAVBAR_ITEMS: TabLinkItem<FilamentPath>[] = [
  {
    id: FilamentPath.OBSERVABILITY,
    label: "Observability",
    href: createFilamentHref(FilamentPath.OBSERVABILITY),
  },
  {
    id: FilamentPath.PIPELINES,
    label: "Pipelines",
    href: createFilamentHref(FilamentPath.PIPELINES),
  },
  { id: FilamentPath.SOURCES, label: "Sources", href: createFilamentHref(FilamentPath.SOURCES) },
  { id: FilamentPath.SINKS, label: "Sinks", href: createFilamentHref(FilamentPath.SINKS) },
];

const MainLayoutNavbarRail: FC<{
  children: ReactNode;
  isEnd?: boolean;
}> = ({ children, isEnd = false }) => (
  <Flex
    grow={1}
    basis={0}
    minWidth={0}
    alignItems={AlignItems.CENTER}
    justifyContent={isEnd ? JustifyContent.END : JustifyContent.START}
    gap={isEnd ? 8 : 12}
    padding={[0, MAIN_LAYOUT_GUTTER]}
  >
    {children}
  </Flex>
);

const MainLayoutNavbar: FC = () => {
  const { session } = useRouteContext({ from: "/_app" });
  const matchRoute = useFilamentMatchRoute();

  const activeItem = MAIN_NAVBAR_ITEMS.find((item) => matchRoute(item.id, { fuzzy: true }));

  return (
    <Box position="relative" height={MAIN_NAVBAR_HEIGHT} fillWidth>
      <Box position="absolute" inset={{ right: 0, bottom: 0, left: 0 }}>
        <Divider />
      </Box>
      <Flex height="100%">
        <MainLayoutNavbarRail>
          <RouterLink href={createFilamentHref(FilamentPath.OBSERVABILITY)}>
            <Flex alignItems={AlignItems.CENTER}>
              <GalaxyFilamentWordmark size={18} />
            </Flex>
          </RouterLink>
          <DocsLink label="Docs" size={TextSize.BODY_SM} underline={LinkUnderline.NONE} />
        </MainLayoutNavbarRail>
        <Tabs
          ariaLabel="Main"
          size={TabsSize.MEDIUM}
          items={MAIN_NAVBAR_ITEMS}
          value={activeItem?.id ?? null}
          as={RouterLink}
        />
        <MainLayoutNavbarRail isEnd>
          {!session.isAuthenticated && <GithubButton />}
          {!session.isAuthenticated && <ThemeSwitcher size={ThemeSwitcherSize.SMALL} isIconOnly />}
          <MainLayoutSettingsButton />
        </MainLayoutNavbarRail>
      </Flex>
    </Box>
  );
};

export default MainLayoutNavbar;
