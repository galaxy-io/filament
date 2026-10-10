import type { Icon } from "@phosphor-icons/react";
import { ClockCounterClockwiseIcon, GearFineIcon, TreeStructureIcon } from "@phosphor-icons/react";

import { PipelineSidebarItem } from "@/layouts/pipeline/types";

import { FilamentPath } from "@/module/paths";

export const PIPELINE_SIDEBAR_WIDTH = 48;
export const PIPELINE_SIDEBAR_BUTTON_SIZE = 28;
export const PIPELINE_NAVBAR_HEIGHT = 48;
export const PIPELINE_NAVBAR_RUN_DROPDOWN_WIDTH = 500;
export const PIPELINE_NAVBAR_VERSION_SELECT_WIDTH = 128;

export const PIPELINE_SIDEBAR_ITEMS: PipelineSidebarItem[] = [
  PipelineSidebarItem.CANVAS,
  PipelineSidebarItem.HISTORY,
  PipelineSidebarItem.SETTINGS,
];

export const PIPELINE_SIDEBAR_ITEM_TO_ICON_MAP: Record<PipelineSidebarItem, Icon> = {
  [PipelineSidebarItem.CANVAS]: TreeStructureIcon,
  [PipelineSidebarItem.HISTORY]: ClockCounterClockwiseIcon,
  [PipelineSidebarItem.SETTINGS]: GearFineIcon,
};

export const PIPELINE_SIDEBAR_ITEM_TO_LABEL_MAP: Record<PipelineSidebarItem, string> = {
  [PipelineSidebarItem.CANVAS]: "Canvas",
  [PipelineSidebarItem.HISTORY]: "History",
  [PipelineSidebarItem.SETTINGS]: "Settings",
};

export const PIPELINE_SIDEBAR_ITEM_TO_PATH_MAP: Record<PipelineSidebarItem, FilamentPath> = {
  [PipelineSidebarItem.CANVAS]: FilamentPath.PIPELINE_CANVAS,
  [PipelineSidebarItem.HISTORY]: FilamentPath.PIPELINE_HISTORY,
  [PipelineSidebarItem.SETTINGS]: FilamentPath.PIPELINE_SETTINGS,
};
