import type { Icon } from "@phosphor-icons/react";
import {
  ClockCounterClockwiseIcon,
  GearIcon,
  TreeStructureIcon,
} from "@phosphor-icons/react";

import { ChipVariant } from "@galaxy-io/dls/chips/Chip";

import {
  PipelineSidebarItem,
  PipelineStatus,
} from "@/layouts/pipeline/types";

// Layout constants
export const PIPELINE_SIDEBAR_WIDTH = 48;
export const PIPELINE_SIDEBAR_BUTTON_SIZE = 28;
export const PIPELINE_NAVBAR_HEIGHT = 48;

// Maps
export const PIPELINE_STATUS_TO_LABEL_MAP: Record<PipelineStatus, string> = {
  [PipelineStatus.DRAFT]: "Draft",
  [PipelineStatus.ACTIVE]: "Active",
  [PipelineStatus.PAUSED]: "Paused",
};

export const PIPELINE_STATUS_TO_CHIP_VARIANT_MAP: Record<PipelineStatus, ChipVariant> = {
  [PipelineStatus.DRAFT]: ChipVariant.SECONDARY,
  [PipelineStatus.ACTIVE]: ChipVariant.SUCCESS,
  [PipelineStatus.PAUSED]: ChipVariant.WARNING,
};

export const PIPELINE_SIDEBAR_ITEM_TO_ICON_MAP: Record<PipelineSidebarItem, Icon> = {
  [PipelineSidebarItem.CANVAS]: TreeStructureIcon,
  [PipelineSidebarItem.HISTORY]: ClockCounterClockwiseIcon,
  [PipelineSidebarItem.SETTINGS]: GearIcon,
};

export const PIPELINE_SIDEBAR_ITEMS: PipelineSidebarItem[] = [
  PipelineSidebarItem.CANVAS,
  PipelineSidebarItem.HISTORY,
  PipelineSidebarItem.SETTINGS,
];
