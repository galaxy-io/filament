import type { Icon } from "@phosphor-icons/react";
import { ClockCounterClockwiseIcon, GearIcon, TreeStructureIcon } from "@phosphor-icons/react";

import { PipelineSidebarItem } from "@/pages/pipelines/layout/types";

export const PIPELINE_SIDEBAR_WIDTH = 48;
export const PIPELINE_SIDEBAR_BUTTON_SIZE = 28;
export const PIPELINE_NAVBAR_HEIGHT = 48;

export const PIPELINE_SIDEBAR_ITEMS: PipelineSidebarItem[] = [
  PipelineSidebarItem.CANVAS,
  PipelineSidebarItem.HISTORY,
  PipelineSidebarItem.SETTINGS,
];

export const PIPELINE_SIDEBAR_ITEM_TO_ICON_MAP: Record<PipelineSidebarItem, Icon> = {
  [PipelineSidebarItem.CANVAS]: TreeStructureIcon,
  [PipelineSidebarItem.HISTORY]: ClockCounterClockwiseIcon,
  [PipelineSidebarItem.SETTINGS]: GearIcon,
};
