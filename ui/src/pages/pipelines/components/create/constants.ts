import type { Icon as PhosphorIcon } from "@phosphor-icons/react";
import { CheckCircleIcon, CircleIcon } from "@phosphor-icons/react";

import { IconVariant, IconWeight } from "@galaxy-io/dls/icons/Icon";
import { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";

import {
  CreatePipelineModalStep,
  CreatePipelineModalStepStatus,
} from "@/pages/pipelines/components/create/types";

export const CREATE_PIPELINE_MODAL_WIDTH = 1080;
export const CREATE_PIPELINE_MODAL_HEIGHT = 720;
export const CREATE_PIPELINE_MODAL_SIDEBAR_WIDTH = 240;

export const CREATE_PIPELINE_MODAL_RESOURCE_LOADING_ROW_COUNT = 8;
export const CREATE_PIPELINE_MODAL_CONNECTION_GHOST_COUNT = 4;
export const CREATE_PIPELINE_MODAL_SINK_SELECT_WIDTH = 264;

export const CREATE_PIPELINE_MODAL_COLUMN_WIDTH_READ_MODE = 180;
export const CREATE_PIPELINE_MODAL_COLUMN_WIDTH_CURSOR = 180;
export const CREATE_PIPELINE_MODAL_READ_MODE_DROPDOWN_WIDTH = 220;
export const CREATE_PIPELINE_MODAL_CURSOR_DROPDOWN_WIDTH = 260;

export const CREATE_PIPELINE_MODAL_STEP_STATUS_TO_ICON_MAP: Record<
  CreatePipelineModalStepStatus,
  PhosphorIcon
> = {
  [CreatePipelineModalStepStatus.COMPLETED]: CheckCircleIcon,
  [CreatePipelineModalStepStatus.CURRENT]: CircleIcon,
  [CreatePipelineModalStepStatus.UPCOMING]: CircleIcon,
};

export const CREATE_PIPELINE_MODAL_STEP_STATUS_TO_ICON_WEIGHT_MAP: Record<
  CreatePipelineModalStepStatus,
  IconWeight
> = {
  [CreatePipelineModalStepStatus.COMPLETED]: IconWeight.FILL,
  [CreatePipelineModalStepStatus.CURRENT]: IconWeight.BOLD,
  [CreatePipelineModalStepStatus.UPCOMING]: IconWeight.REGULAR,
};

export const CREATE_PIPELINE_MODAL_STEP_STATUS_TO_ICON_VARIANT_MAP: Record<
  CreatePipelineModalStepStatus,
  IconVariant
> = {
  [CreatePipelineModalStepStatus.COMPLETED]: IconVariant.SUCCESS,
  [CreatePipelineModalStepStatus.CURRENT]: IconVariant.PRIMARY,
  [CreatePipelineModalStepStatus.UPCOMING]: IconVariant.DISABLED,
};

export const CREATE_PIPELINE_MODAL_STEP_STATUS_TO_TEXT_VARIANT_MAP: Record<
  CreatePipelineModalStepStatus,
  TextVariant
> = {
  [CreatePipelineModalStepStatus.COMPLETED]: TextVariant.PRIMARY,
  [CreatePipelineModalStepStatus.CURRENT]: TextVariant.PRIMARY,
  [CreatePipelineModalStepStatus.UPCOMING]: TextVariant.TERTIARY,
};

export const CREATE_PIPELINE_MODAL_STEP_STATUS_TO_TEXT_WEIGHT_MAP: Record<
  CreatePipelineModalStepStatus,
  TextWeight
> = {
  [CreatePipelineModalStepStatus.COMPLETED]: TextWeight.REGULAR,
  [CreatePipelineModalStepStatus.CURRENT]: TextWeight.MEDIUM,
  [CreatePipelineModalStepStatus.UPCOMING]: TextWeight.REGULAR,
};

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
    "Choose a source to pull data from and one or more sinks to deliver it to.",
  [CreatePipelineModalStep.RESOURCES]:
    "Pick the resources you want to ingest and how each one is read and written.",
  [CreatePipelineModalStep.DELIVERY]:
    "Set a schedule so the pipeline runs on its own, or leave it manual and trigger runs yourself.",
  [CreatePipelineModalStep.DETAILS]:
    "Name your pipeline and add an optional description so your team knows what it does.",
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
};

export const CREATE_PIPELINE_MODAL_DEFAULT_READ_MODE = ReadMode.FULL;
export const CREATE_PIPELINE_MODAL_DEFAULT_WRITE_MODE = WriteMode.REPLACE;
