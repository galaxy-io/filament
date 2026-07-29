import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  useContext,
  useMemo,
  useReducer,
} from "react";

import type { RunBinding } from "@/gen/ingestion/v1/runs_pb";

import {
  type PipelineCanvasRunAction,
  PipelineCanvasRunActionType,
} from "@/pages/pipelines/canvas/providers/run/actions";
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

const usePipelineCanvasRunDispatch = () => {
  const dispatch = useContext(PipelineCanvasRunDispatchContext);
  if (!dispatch) {
    throw new Error("usePipelineCanvasRunDispatch must be used within PipelineCanvasRunProvider");
  }
  return dispatch;
};

export const usePipelineCanvasRunActions = () => {
  const dispatch = usePipelineCanvasRunDispatch();

  return useMemo(
    () => ({
      startRun: (runBindings: RunBinding[]) =>
        dispatch({ type: PipelineCanvasRunActionType.START_RUN, payload: runBindings }),
      setActivityOpen: (isActivityOpen: boolean) =>
        dispatch({
          type: PipelineCanvasRunActionType.SET_ACTIVITY_OPEN,
          payload: isActivityOpen,
        }),
    }),
    [dispatch],
  );
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
