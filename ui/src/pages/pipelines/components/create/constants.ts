import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { CircleIcon, InfinityIcon, StackIcon } from "@phosphor-icons/react";

import { ExecutionMode, ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";

import { CreatePipelineModalStep } from "@/pages/pipelines/components/create/types";

export const CREATE_PIPELINE_MODAL_SIDEBAR_WIDTH = 240;

export const CREATE_PIPELINE_MODAL_CONNECTION_GHOST_COUNT = 4;
export const CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH = 264;
export const CREATE_PIPELINE_MODAL_CONNECTION_ROW_HEIGHT = 40;
export const CREATE_PIPELINE_MODAL_CONNECTION_TOGGLE_WIDTH = 24;

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

export const CREATE_PIPELINE_MODAL_STEP_TO_DESCRIPTION_MAP: Record<
  CreatePipelineModalStep,
  string
> = {
  [CreatePipelineModalStep.CONNECTIONS]:
    "Pick a source to read from and one or more sinks to write to.",
  [CreatePipelineModalStep.RESOURCES]:
    "Pick the resources you want to ingest and how each one is read and written.",
  [CreatePipelineModalStep.DELIVERY]: "Choose how each sink is written and who gets notified.",
  [CreatePipelineModalStep.DETAILS]:
    "Name your pipeline and add an optional description so your team knows what it does.",
};

export const CREATE_PIPELINE_MODAL_STEP_TO_HINT_MAP: Record<CreatePipelineModalStep, string> = {
  [CreatePipelineModalStep.CONNECTIONS]: "Select a source and at least one sink",
  [CreatePipelineModalStep.RESOURCES]: "Select at least one resource",
  [CreatePipelineModalStep.DELIVERY]: "Complete the schedule to continue",
  [CreatePipelineModalStep.DETAILS]: "Enter a valid pipeline name",
};

export const EXECUTION_MODE_TO_ICON_MAP: Record<ExecutionMode, PhosphorIcon> = {
  [ExecutionMode.UNSPECIFIED]: CircleIcon,
  [ExecutionMode.BOUNDED]: StackIcon,
  [ExecutionMode.CONTINUOUS]: InfinityIcon,
};

export const EXECUTION_MODE_TO_LABEL_MAP: Record<ExecutionMode, string> = {
  [ExecutionMode.UNSPECIFIED]: "Unknown",
  [ExecutionMode.BOUNDED]: "Batch",
  [ExecutionMode.CONTINUOUS]: "Continuous",
};

export const EXECUTION_MODE_TO_DESCRIPTION_MAP: Record<ExecutionMode, string> = {
  [ExecutionMode.UNSPECIFIED]: "",
  [ExecutionMode.BOUNDED]: "Reads available data then stops.",
  [ExecutionMode.CONTINUOUS]: "Streams new data until stopped.",
};

export const EXECUTION_MODE_TO_DETAILS_MAP: Record<ExecutionMode, string> = {
  [ExecutionMode.UNSPECIFIED]: "",
  [ExecutionMode.BOUNDED]:
    "Reads available data then stops. Runs on a schedule or on demand, with full or incremental reads.",
  [ExecutionMode.CONTINUOUS]:
    "Streams new data until you stop it. Saves progress in epochs and appends records to each sink.",
};

export const READ_MODE_TO_LABEL_MAP: Record<ReadMode, string> = {
  [ReadMode.UNSPECIFIED]: "Unknown",
  [ReadMode.FULL]: "Full",
  [ReadMode.INCREMENTAL]: "Incremental",
  [ReadMode.CDC]: "CDC",
};

export const WRITE_MODE_TO_LABEL_MAP: Record<WriteMode, string> = {
  [WriteMode.UNSPECIFIED]: "Unknown",
  [WriteMode.APPEND]: "Append",
  [WriteMode.REPLACE]: "Replace",
  [WriteMode.UPSERT]: "Upsert",
  [WriteMode.MERGE]: "Merge",
  [WriteMode.DELETE]: "Delete",
};

export const CREATE_PIPELINE_MODAL_DEFAULT_READ_MODE = ReadMode.FULL;
export const CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE = WriteMode.UPSERT;

export const CREATE_PIPELINE_MODAL_DEFAULT_CRON = "0 * * * *";
