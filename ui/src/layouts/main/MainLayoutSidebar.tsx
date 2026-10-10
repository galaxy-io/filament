import type { FC, ReactNode } from "react";

import PoweredBy, { PoweredByWordmark } from "@galaxy-io/dls/brand/PoweredBy";
import SidebarNav, { NavItem } from "@galaxy-io/dls/navigation/SidebarNav";

import RouterLink from "@/components/RouterLink";

import { useFilamentMatchRoute } from "@/module/hooks";
import {
  FILAMENT_NAV_ITEM_TO_ICON_MAP,
  FILAMENT_NAV_ITEM_TO_LABEL_MAP,
  FILAMENT_NAV_ITEM_TO_PATH_MAP,
  FILAMENT_NAV_ITEMS,
} from "@/module/nav";
import { createFilamentHref } from "@/module/paths";

import { GITHUB_REPO_URL } from "@/constants";

interface MainLayoutSidebarProps {
  footer?: ReactNode;
}

const MainLayoutSidebar: FC<MainLayoutSidebarProps> = ({ footer }) => {
  const matchRoute = useFilamentMatchRoute();

  const activeItem = FILAMENT_NAV_ITEMS.find((item) =>
    matchRoute(FILAMENT_NAV_ITEM_TO_PATH_MAP[item], { fuzzy: true }),
  );

  return (
    <SidebarNav
      ariaLabel="Filament"
      hasDividers={false}
      footer={
        <>
          {footer}
          <PoweredBy
            wordmark={PoweredByWordmark.FILAMENT}
            href={GITHUB_REPO_URL}
            ariaLabel="Filament on GitHub"
          />
        </>
      }
    >
      {FILAMENT_NAV_ITEMS.map((item) => (
        <NavItem
          key={item}
          label={FILAMENT_NAV_ITEM_TO_LABEL_MAP[item]}
          icon={FILAMENT_NAV_ITEM_TO_ICON_MAP[item]}
          href={createFilamentHref(FILAMENT_NAV_ITEM_TO_PATH_MAP[item])}
          as={RouterLink}
          isActive={activeItem === item}
        />
      ))}
    </SidebarNav>
  );
};

export default MainLayoutSidebar;
