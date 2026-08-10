export enum PipelineCanvasRunActionType {
  SET_ACTIVITY_OPEN = "SET_ACTIVITY_OPEN",
}

export interface SetActivityOpenAction {
  type: PipelineCanvasRunActionType.SET_ACTIVITY_OPEN;
  payload: boolean;
}

export type PipelineCanvasRunAction = SetActivityOpenAction;
