export interface NavItem {
  to: string;
  label: string;
}

export const NAV_ITEMS: NavItem[] = [
  { to: "/pipelines", label: "Pipelines" },
  { to: "/sources", label: "Sources" },
  { to: "/sinks", label: "Sinks" },
  { to: "/logs", label: "Logs" },
];
