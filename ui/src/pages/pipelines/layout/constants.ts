import type { Icon } from "@phosphor-icons/react";
import { ClockCounterClockwiseIcon, GearFineIcon, TreeStructureIcon } from "@phosphor-icons/react";

import { PipelineSidebarItem } from "@/pages/pipelines/layout/types";

export const PIPELINE_SIDEBAR_WIDTH = 48;
export const PIPELINE_SIDEBAR_BUTTON_SIZE = 28;
export const PIPELINE_NAVBAR_HEIGHT = 48;
export const PIPELINE_PREVIEW_CHIP_Z_INDEX = 1002;
export const PIPELINE_VERSION_SELECT_DROPDOWN_WIDTH = 200;
export const PIPELINE_NAVBAR_RUN_DROPDOWN_WIDTH = 500;

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
