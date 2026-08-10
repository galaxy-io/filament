import {
  type PipelineCanvasRunAction,
  PipelineCanvasRunActionType,
  type SetActivityOpenAction,
} from "@/pages/pipelines/canvas/providers/run/actions";
import type { PipelineCanvasRunState } from "@/pages/pipelines/canvas/providers/run/types";

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
    case PipelineCanvasRunActionType.SET_ACTIVITY_OPEN:
      return setActivityOpen(state, action);
  }
};

export default pipelineCanvasRunReducer;
