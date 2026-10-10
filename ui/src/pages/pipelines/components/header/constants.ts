import type { Icon } from "@phosphor-icons/react";
import {
  ClockCounterClockwiseIcon,
  FunctionIcon,
  GearFineIcon,
  TreeStructureIcon,
  WarningIcon,
} from "@phosphor-icons/react";

import {
  type PipelineCanvasValidationIssue,
  PipelineCanvasValidationIssueKind,
} from "@/pages/pipelines/canvas/hooks/usePipelineCanvasValidation";
import { PipelinePageTab } from "@/pages/pipelines/components/header/types";

import { FilamentPath } from "@/module/paths";

export const PIPELINE_PAGE_TABS_INSET = 16;
export const PIPELINE_PAGE_VERSION_SELECT_WIDTH = 128;
export const PIPELINE_PAGE_RUN_DROPDOWN_WIDTH = 500;
export const PIPELINE_PAGE_MAX_VISIBLE_SAVE_ISSUES = 3;

export const PIPELINE_PAGE_TABS: PipelinePageTab[] = [
  PipelinePageTab.CANVAS,
  PipelinePageTab.HISTORY,
  PipelinePageTab.SETTINGS,
];

export const PIPELINE_PAGE_TAB_TO_LABEL_MAP: Record<PipelinePageTab, string> = {
  [PipelinePageTab.CANVAS]: "Canvas",
  [PipelinePageTab.HISTORY]: "History",
  [PipelinePageTab.SETTINGS]: "Settings",
};

export const PIPELINE_PAGE_TAB_TO_ICON_MAP: Record<PipelinePageTab, Icon> = {
  [PipelinePageTab.CANVAS]: TreeStructureIcon,
  [PipelinePageTab.HISTORY]: ClockCounterClockwiseIcon,
  [PipelinePageTab.SETTINGS]: GearFineIcon,
};

export const PIPELINE_PAGE_TAB_TO_PATH_MAP: Record<PipelinePageTab, FilamentPath> = {
  [PipelinePageTab.CANVAS]: FilamentPath.PIPELINE_CANVAS,
  [PipelinePageTab.HISTORY]: FilamentPath.PIPELINE_HISTORY,
  [PipelinePageTab.SETTINGS]: FilamentPath.PIPELINE_SETTINGS,
};

export const PIPELINE_PAGE_SAVE_ISSUE_KIND_TO_ICON_MAP: Record<
  PipelineCanvasValidationIssueKind,
  Icon
> = {
  [PipelineCanvasValidationIssueKind.TRANSFORM]: FunctionIcon,
  [PipelineCanvasValidationIssueKind.EDGE]: WarningIcon,
  [PipelineCanvasValidationIssueKind.GRAPH]: WarningIcon,
};

export const PIPELINE_PAGE_SAVE_ISSUE_KIND_TO_LABEL_FIELD_MAP: Record<
  PipelineCanvasValidationIssueKind,
  keyof Pick<PipelineCanvasValidationIssue, "resource" | "message">
> = {
  [PipelineCanvasValidationIssueKind.TRANSFORM]: "resource",
  [PipelineCanvasValidationIssueKind.EDGE]: "message",
  [PipelineCanvasValidationIssueKind.GRAPH]: "message",
};
