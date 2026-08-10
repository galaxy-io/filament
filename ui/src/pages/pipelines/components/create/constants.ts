import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalStep } from "@/pages/pipelines/components/create/types";

export const CREATE_PIPELINE_MODAL_WIDTH = 1080;
export const CREATE_PIPELINE_MODAL_HEIGHT = 720;
export const CREATE_PIPELINE_MODAL_SIDEBAR_WIDTH = 240;

export const CREATE_PIPELINE_MODAL_RESOURCE_LOADING_ROW_COUNT = 8;
export const CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH = 264;

export const CREATE_PIPELINE_MODAL_COLUMN_WIDTH_READ_MODE = 180;
export const CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR = 180;

export const CREATE_PIPELINE_MODAL_STEP_ORDER: CreatePipelineModalStep[] = [
  CreatePipelineModalStep.CONNECTIONS,
  CreatePipelineModalStep.RESOURCES,
  CreatePipelineModalStep.DELIVERY,
  CreatePipelineModalStep.DETAILS,
];

export const CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP: Record<CreatePipelineModalStep, string> = {
  [CreatePipelineModalStep.CONNECTIONS]: "Choose connections",
  [CreatePipelineModalStep.RESOURCES]: "Select resources",
  [CreatePipelineModalStep.DELIVERY]: "Configure delivery",
  [CreatePipelineModalStep.DETAILS]: "Name pipeline",
};

export const CREATE_PIPELINE_MODAL_STEP_TO_IS_PADDED_MAP: Record<CreatePipelineModalStep, boolean> =
  {
    [CreatePipelineModalStep.CONNECTIONS]: false,
    [CreatePipelineModalStep.RESOURCES]: false,
    [CreatePipelineModalStep.DELIVERY]: true,
    [CreatePipelineModalStep.DETAILS]: true,
  };

export const CREATE_PIPELINE_MODAL_STEP_TO_HINT_MAP: Record<CreatePipelineModalStep, string> = {
  [CreatePipelineModalStep.CONNECTIONS]: "Select a source and at least one sink",
  [CreatePipelineModalStep.RESOURCES]: "Select at least one resource",
  [CreatePipelineModalStep.DELIVERY]: "Complete the schedule to continue",
  [CreatePipelineModalStep.DETAILS]: "Enter a valid pipeline name",
};

export const READ_MODE_TO_LABEL_MAP: Record<ReadMode, string> = {
  [ReadMode.UNSPECIFIED]: "Unknown",
  [ReadMode.FULL]: "Full",
  [ReadMode.INCREMENTAL]: "Incremental",
};

export const WRITE_MODE_TO_LABEL_MAP: Record<WriteMode, string> = {
  [WriteMode.UNSPECIFIED]: "Unknown",
  [WriteMode.APPEND]: "Append",
  [WriteMode.REPLACE]: "Replace",
  [WriteMode.UPSERT]: "Upsert",
  [WriteMode.DELETE]: "Delete",
  [WriteMode.MERGE]: "Merge",
};

export const READ_MODE_TO_WRITE_MODES_MAP: Record<ReadMode, WriteMode[]> = {
  [ReadMode.UNSPECIFIED]: [WriteMode.REPLACE, WriteMode.APPEND, WriteMode.UPSERT],
  [ReadMode.FULL]: [WriteMode.REPLACE, WriteMode.APPEND, WriteMode.UPSERT],
  [ReadMode.INCREMENTAL]: [WriteMode.APPEND, WriteMode.UPSERT],
};

export const CREATE_PIPELINE_MODAL_FALLBACK_READ_MODES = [ReadMode.FULL];
export const CREATE_PIPELINE_MODAL_FALLBACK_WRITE_MODES = [WriteMode.APPEND, WriteMode.REPLACE];

export const CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE = WriteMode.REPLACE;
