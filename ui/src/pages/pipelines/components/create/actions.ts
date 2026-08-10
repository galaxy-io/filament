import type { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";

import type { CreatePipelineModalStep } from "@/pages/pipelines/components/create/types";
import type { PipelineSettingsPageScheduleState } from "@/pages/pipelines/settings/types";

export enum CreatePipelineModalActionType {
  SELECT_SOURCE = "SELECT_SOURCE",
  TOGGLE_SINK = "TOGGLE_SINK",
  SET_ACTIVE_SINK = "SET_ACTIVE_SINK",
  OPEN_SINK_RESOURCES = "OPEN_SINK_RESOURCES",
  SET_RESOURCE_SELECTION = "SET_RESOURCE_SELECTION",
  SET_RESOURCE_READ_MODE = "SET_RESOURCE_READ_MODE",
  SET_RESOURCE_CURSOR = "SET_RESOURCE_CURSOR",
  SET_SINK_WRITE_MODE = "SET_SINK_WRITE_MODE",
  SET_NAME = "SET_NAME",
  SET_DESCRIPTION = "SET_DESCRIPTION",
  SET_SCHEDULE = "SET_SCHEDULE",
  GO_TO_STEP = "GO_TO_STEP",
  GO_BACK = "GO_BACK",
  GO_NEXT = "GO_NEXT",
  SET_SUBMITTING = "SET_SUBMITTING",
}

export interface SelectSourceAction {
  type: CreatePipelineModalActionType.SELECT_SOURCE;
  payload: Connection;
}

export interface ToggleSinkAction {
  type: CreatePipelineModalActionType.TOGGLE_SINK;
  payload: Connection;
}

export interface SetActiveSinkAction {
  type: CreatePipelineModalActionType.SET_ACTIVE_SINK;
  payload: string;
}

export interface OpenSinkResourcesAction {
  type: CreatePipelineModalActionType.OPEN_SINK_RESOURCES;
  payload: string;
}

export interface SetResourceSelectionAction {
  type: CreatePipelineModalActionType.SET_RESOURCE_SELECTION;
  payload: { sinkId: string; visibleNames: string[]; selection: Record<string, boolean> };
}

export interface SetResourceReadModeAction {
  type: CreatePipelineModalActionType.SET_RESOURCE_READ_MODE;
  payload: { sinkId: string; resource: string; readMode: ReadMode };
}

export interface SetResourceCursorAction {
  type: CreatePipelineModalActionType.SET_RESOURCE_CURSOR;
  payload: { sinkId: string; resource: string; cursorField: string };
}

export interface SetSinkWriteModeAction {
  type: CreatePipelineModalActionType.SET_SINK_WRITE_MODE;
  payload: { sinkId: string; writeMode: WriteMode };
}

export interface SetNameAction {
  type: CreatePipelineModalActionType.SET_NAME;
  payload: string;
}

export interface SetDescriptionAction {
  type: CreatePipelineModalActionType.SET_DESCRIPTION;
  payload: string;
}

export interface SetScheduleAction {
  type: CreatePipelineModalActionType.SET_SCHEDULE;
  payload: Partial<PipelineSettingsPageScheduleState>;
}

export interface GoToStepAction {
  type: CreatePipelineModalActionType.GO_TO_STEP;
  payload: CreatePipelineModalStep;
}

export interface GoBackAction {
  type: CreatePipelineModalActionType.GO_BACK;
}

export interface GoNextAction {
  type: CreatePipelineModalActionType.GO_NEXT;
}

export interface SetSubmittingAction {
  type: CreatePipelineModalActionType.SET_SUBMITTING;
  payload: boolean;
}

export type CreatePipelineModalAction =
  | SelectSourceAction
  | ToggleSinkAction
  | SetActiveSinkAction
  | OpenSinkResourcesAction
  | SetResourceSelectionAction
  | SetResourceReadModeAction
  | SetResourceCursorAction
  | SetSinkWriteModeAction
  | SetNameAction
  | SetDescriptionAction
  | SetScheduleAction
  | GoToStepAction
  | GoBackAction
  | GoNextAction
  | SetSubmittingAction;
