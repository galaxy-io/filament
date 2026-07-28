import {
  type PipelineCanvasRunAction,
  type SetActivityOpenAction,
  type StartRunAction,
  PipelineCanvasRunActionType,
} from "@/pages/pipelines/canvas/providers/run/actions";
import type { PipelineCanvasRunState } from "@/pages/pipelines/canvas/providers/run/types";

function startRun(
  state: PipelineCanvasRunState,
  action: StartRunAction,
): PipelineCanvasRunState {
  return { ...state, runBindings: action.payload, isActivityOpen: true };
}

function setActivityOpen(
  state: PipelineCanvasRunState,
  action: SetActivityOpenAction,
): PipelineCanvasRunState {
  return { ...state, isActivityOpen: action.payload };
}

const pipelineCanvasRunReducer = (
  state: PipelineCanvasRunState,
  action: PipelineCanvasRunAction,
): PipelineCanvasRunState => {
  switch (action.type) {
    case PipelineCanvasRunActionType.START_RUN:
      return startRun(state, action);
    case PipelineCanvasRunActionType.SET_ACTIVITY_OPEN:
      return setActivityOpen(state, action);
  }
};

export default pipelineCanvasRunReducer;
