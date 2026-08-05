import { CreatePipelineModalStep } from "@/pages/pipelines/components/create/types";

export const CREATE_PIPELINE_MODAL_WIDTH = 960;
export const CREATE_PIPELINE_MODAL_SIDEBAR_WIDTH = 240;
export const CREATE_PIPELINE_MODAL_MIN_HEIGHT = 640;
export const CREATE_PIPELINE_MODAL_MAX_HEIGHT = 720;

export const CREATE_PIPELINE_MODAL_STEP_ORDER: CreatePipelineModalStep[] = [
  CreatePipelineModalStep.CONNECTIONS,
  CreatePipelineModalStep.DETAILS,
  CreatePipelineModalStep.SCHEDULE,
];

export const CREATE_PIPELINE_MODAL_STEP_TO_TITLE_MAP: Record<CreatePipelineModalStep, string> = {
  [CreatePipelineModalStep.CONNECTIONS]: "Choose your connections",
  [CreatePipelineModalStep.DETAILS]: "Configure your pipeline",
  [CreatePipelineModalStep.SCHEDULE]: "Set your schedule",
};

export const CREATE_PIPELINE_MODAL_STEP_TO_DESCRIPTION_MAP: Record<
  CreatePipelineModalStep,
  string
> = {
  [CreatePipelineModalStep.CONNECTIONS]: "Choose a source and sinks",
  [CreatePipelineModalStep.DETAILS]: "Name your pipeline",
  [CreatePipelineModalStep.SCHEDULE]: "Set an optional schedule",
};
