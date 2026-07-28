import type { RunBinding } from "@/gen/ingestion/v1/runs_pb";

export enum PipelineCanvasRunActionType {
  START_RUN = "START_RUN",
  SET_ACTIVITY_OPEN = "SET_ACTIVITY_OPEN",
}

export interface StartRunAction {
  type: PipelineCanvasRunActionType.START_RUN;
  payload: RunBinding[];
}

export interface SetActivityOpenAction {
  type: PipelineCanvasRunActionType.SET_ACTIVITY_OPEN;
  payload: boolean;
}

export type PipelineCanvasRunAction = StartRunAction | SetActivityOpenAction;
