import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  useContext,
  useReducer,
} from "react";

import type { PipelineCanvasRunAction } from "@/pages/pipelines/canvas/providers/run/actions";
import pipelineCanvasRunReducer from "@/pages/pipelines/canvas/providers/run/reducer";
import type { PipelineCanvasRunState } from "@/pages/pipelines/canvas/providers/run/types";

const DEFAULT_STATE: PipelineCanvasRunState = {
  runBindings: [],
  isActivityOpen: false,
};

const PipelineCanvasRunStateContext = createContext<PipelineCanvasRunState | null>(null);
PipelineCanvasRunStateContext.displayName = "PipelineCanvasRunStateContext";

const PipelineCanvasRunDispatchContext = createContext<Dispatch<PipelineCanvasRunAction> | null>(
  null,
);
PipelineCanvasRunDispatchContext.displayName = "PipelineCanvasRunDispatchContext";

export const usePipelineCanvasRunState = () => {
  const state = useContext(PipelineCanvasRunStateContext);
  if (!state) {
    throw new Error("usePipelineCanvasRunState must be used within PipelineCanvasRunProvider");
  }
  return state;
};

export const usePipelineCanvasRunDispatch = () => {
  const dispatch = useContext(PipelineCanvasRunDispatchContext);
  if (!dispatch) {
    throw new Error("usePipelineCanvasRunDispatch must be used within PipelineCanvasRunProvider");
  }
  return dispatch;
};

const PipelineCanvasRunProvider = ({ children }: PropsWithChildren) => {
  const [state, dispatch] = useReducer(pipelineCanvasRunReducer, DEFAULT_STATE);

  return (
    <PipelineCanvasRunDispatchContext.Provider value={dispatch}>
      <PipelineCanvasRunStateContext.Provider value={state}>
        {children}
      </PipelineCanvasRunStateContext.Provider>
    </PipelineCanvasRunDispatchContext.Provider>
  );
};

export default PipelineCanvasRunProvider;
