import { PipelineRunResourceMode } from "@/pages/pipelines/components/run/types";

export const PIPELINE_RUN_MODAL_TITLE_ICON_SIZE = 15;
export const PIPELINE_RUN_MODAL_RESOURCES_CARET_SIZE = 12;
export const PIPELINE_RUN_MODAL_RESOURCES_PANE_WIDTH = 420;
export const PIPELINE_RUN_MODAL_RESOURCE_MODE_COLUMN_WIDTH = 130;

export const PIPELINE_RUN_RESOURCE_MODE_OPTIONS = [
  { id: PipelineRunResourceMode.CONTINUE, label: "Continue" },
  { id: PipelineRunResourceMode.RESYNC, label: "Resync" },
];
