import type { Icon } from "@phosphor-icons/react";
import {
  ClockCounterClockwiseIcon,
  GearFineIcon,
  PauseIcon,
  PlayIcon,
  TreeStructureIcon,
} from "@phosphor-icons/react";

import { Signal } from "@/gen/ingestion/v1/runs_pb";

import { PipelineSidebarItem } from "@/pages/pipelines/layout/types";

export const PIPELINE_SIDEBAR_WIDTH = 48;
export const PIPELINE_SIDEBAR_BUTTON_SIZE = 28;
export const PIPELINE_NAVBAR_HEIGHT = 48;
export const PIPELINE_PREVIEW_CHIP_Z_INDEX = 1002;
export const PIPELINE_VERSION_SELECT_DROPDOWN_WIDTH = 200;

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

export const PIPELINE_RUN_PAUSE_ACTION = { label: "Pause", icon: PauseIcon, signal: Signal.PAUSE };
export const PIPELINE_RUN_RESUME_ACTION = {
  label: "Resume",
  icon: PlayIcon,
  signal: Signal.RESUME,
};
