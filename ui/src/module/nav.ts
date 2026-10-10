import {
  ChartLineIcon,
  DatabaseIcon,
  FlowArrowIcon,
  type Icon,
  TrayArrowDownIcon,
} from "@phosphor-icons/react";

import { FilamentPath } from "@/module/paths";
import { FilamentNavItem } from "@/module/types";

export const FILAMENT_NAV_ITEMS: FilamentNavItem[] = [
  FilamentNavItem.OBSERVABILITY,
  FilamentNavItem.PIPELINES,
  FilamentNavItem.SOURCES,
  FilamentNavItem.SINKS,
];

export const FILAMENT_NAV_ITEM_TO_LABEL_MAP: Record<FilamentNavItem, string> = {
  [FilamentNavItem.OBSERVABILITY]: "Observability",
  [FilamentNavItem.PIPELINES]: "Pipelines",
  [FilamentNavItem.SOURCES]: "Sources",
  [FilamentNavItem.SINKS]: "Sinks",
};

export const FILAMENT_NAV_ITEM_TO_ICON_MAP: Record<FilamentNavItem, Icon> = {
  [FilamentNavItem.OBSERVABILITY]: ChartLineIcon,
  [FilamentNavItem.PIPELINES]: FlowArrowIcon,
  [FilamentNavItem.SOURCES]: DatabaseIcon,
  [FilamentNavItem.SINKS]: TrayArrowDownIcon,
};

export const FILAMENT_NAV_ITEM_TO_PATH_MAP: Record<FilamentNavItem, FilamentPath> = {
  [FilamentNavItem.OBSERVABILITY]: FilamentPath.OBSERVABILITY,
  [FilamentNavItem.PIPELINES]: FilamentPath.PIPELINES,
  [FilamentNavItem.SOURCES]: FilamentPath.SOURCES,
  [FilamentNavItem.SINKS]: FilamentPath.SINKS,
};

export const FILAMENT_NAV_ITEM_TO_KEYWORDS_MAP: Record<FilamentNavItem, string[]> = {
  [FilamentNavItem.OBSERVABILITY]: ["overview", "dashboard", "metrics", "runs"],
  [FilamentNavItem.PIPELINES]: ["replication", "sync", "jobs"],
  [FilamentNavItem.SOURCES]: ["connections", "connectors", "databases", "apis"],
  [FilamentNavItem.SINKS]: ["destinations", "warehouses", "connections"],
};

export const FILAMENT_NAV_ITEM_TO_COMMAND_ID_MAP: Record<FilamentNavItem, string> = {
  [FilamentNavItem.OBSERVABILITY]: "go-observability",
  [FilamentNavItem.PIPELINES]: "go-pipelines",
  [FilamentNavItem.SOURCES]: "go-sources",
  [FilamentNavItem.SINKS]: "go-sinks",
};
