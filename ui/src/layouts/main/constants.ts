// Layout constants
export const NAVBAR_HEIGHT = 52;

export interface NavItem {
  to: string;
  label: string;
}

export const NAV_ITEMS: NavItem[] = [
  { to: "/pipelines", label: "Pipelines" },
  { to: "/providers", label: "Providers" },
];
