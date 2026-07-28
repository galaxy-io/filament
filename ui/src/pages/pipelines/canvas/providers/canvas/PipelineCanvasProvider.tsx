import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  useContext,
  useReducer,
} from "react";

import type { PipelineCanvasAction } from "@/pages/pipelines/canvas/providers/canvas/actions";
import pipelineCanvasReducer from "@/pages/pipelines/canvas/providers/canvas/reducer";
import {
  PipelineCanvasInteractionMode,
  type PipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/types";

const DEFAULT_STATE: PipelineCanvasState = {
  nodes: [],
  edges: [],
  activeMode: null,
  interactionMode: PipelineCanvasInteractionMode.GRAB,
};

const PipelineCanvasStateContext = createContext<PipelineCanvasState | null>(null);
PipelineCanvasStateContext.displayName = "PipelineCanvasStateContext";

const PipelineCanvasDispatchContext = createContext<Dispatch<PipelineCanvasAction> | null>(null);
PipelineCanvasDispatchContext.displayName = "PipelineCanvasDispatchContext";

const PipelineCanvasReadOnlyContext = createContext<boolean>(false);
PipelineCanvasReadOnlyContext.displayName = "PipelineCanvasReadOnlyContext";

export const usePipelineCanvasState = () => {
  const state = useContext(PipelineCanvasStateContext);
  if (!state) {
    throw new Error("usePipelineCanvasState must be used within PipelineCanvasProvider");
  }
  return state;
};

export const usePipelineCanvasDispatch = () => {
  const dispatch = useContext(PipelineCanvasDispatchContext);
  if (!dispatch) {
    throw new Error("usePipelineCanvasDispatch must be used within PipelineCanvasProvider");
  }
  return dispatch;
};

export const usePipelineCanvasReadOnly = () => useContext(PipelineCanvasReadOnlyContext);

interface PipelineCanvasProviderProps {
  initialState?: Partial<PipelineCanvasState>;
  isReadOnly?: boolean;
}

const PipelineCanvasProvider = ({
  children,
  initialState,
  isReadOnly = false,
}: PropsWithChildren<PipelineCanvasProviderProps>) => {
  const [state, dispatch] = useReducer(pipelineCanvasReducer, {
    ...DEFAULT_STATE,
    ...initialState,
  });

  return (
    <PipelineCanvasReadOnlyContext.Provider value={isReadOnly}>
      <PipelineCanvasDispatchContext.Provider value={dispatch}>
        <PipelineCanvasStateContext.Provider value={state}>
          {children}
        </PipelineCanvasStateContext.Provider>
      </PipelineCanvasDispatchContext.Provider>
    </PipelineCanvasReadOnlyContext.Provider>
  );
};

export default PipelineCanvasProvider;
