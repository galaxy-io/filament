import type { ReadMode, WriteMode } from "@/gen/ingestion/v1/common_pb";
import type { Connection } from "@/gen/ingestion/v1/connections_pb";
import type { Resource, ResourceColumn } from "@/gen/ingestion/v1/connectors_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";

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
  SET_WORKER_CONFIGURATION = "SET_WORKER_CONFIGURATION",
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
  payload: Connection["id"];
}

export interface OpenSinkResourcesAction {
  type: CreatePipelineModalActionType.OPEN_SINK_RESOURCES;
  payload: Connection["id"];
}

export interface SetResourceSelectionAction {
  type: CreatePipelineModalActionType.SET_RESOURCE_SELECTION;
  payload: {
    sinkId: Connection["id"];
    visibleNames: Resource["name"][];
    selection: Record<Resource["name"], boolean>;
  };
}

export interface SetResourceReadModeAction {
  type: CreatePipelineModalActionType.SET_RESOURCE_READ_MODE;
  payload: { sinkId: Connection["id"]; resource: Resource["name"]; readMode: ReadMode };
}

export interface SetResourceCursorAction {
  type: CreatePipelineModalActionType.SET_RESOURCE_CURSOR;
  payload: {
    sinkId: Connection["id"];
    resource: Resource["name"];
    cursorField: ResourceColumn["name"];
  };
}

export interface SetSinkWriteModeAction {
  type: CreatePipelineModalActionType.SET_SINK_WRITE_MODE;
  payload: { sinkId: Connection["id"]; writeMode: WriteMode };
}

export interface SetNameAction {
  type: CreatePipelineModalActionType.SET_NAME;
  payload: Pipeline["name"];
}

export interface SetDescriptionAction {
  type: CreatePipelineModalActionType.SET_DESCRIPTION;
  payload: Pipeline["description"];
}

export interface SetScheduleAction {
  type: CreatePipelineModalActionType.SET_SCHEDULE;
  payload: Partial<PipelineSettingsPageScheduleState>;
}

export interface SetWorkerConfigurationAction {
  type: CreatePipelineModalActionType.SET_WORKER_CONFIGURATION;
  payload: string;
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
  | SetWorkerConfigurationAction
  | GoToStepAction
  | GoBackAction
  | GoNextAction
  | SetSubmittingAction;
