import type { TRoutes } from "@/hooks/useRouteMatch";

export const NAVBAR_HEIGHT = 52;

export interface NavItem {
  to: TRoutes;
  label: string;
}

export const NAV_ITEMS: NavItem[] = [
  { to: "/pipelines", label: "Pipelines" },
  { to: "/sources", label: "Sources" },
  { to: "/sinks", label: "Sinks" },
];
