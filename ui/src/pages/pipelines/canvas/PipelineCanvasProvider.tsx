import { createContext, type PropsWithChildren, useReducer } from "react";

import pipelineCanvasReducer from "@/pages/pipelines/canvas/reducer";
import type {
  PipelineCanvasContextShape,
  PipelineCanvasState,
} from "@/pages/pipelines/canvas/types";

export const DEFAULT_STATE: PipelineCanvasState = {
  nodes: [],
  edges: [],
  activeMode: null,
};

const DEFAULT_CONTEXT: PipelineCanvasContextShape = {
  state: DEFAULT_STATE,
  dispatch: () => undefined,
};

export const PipelineCanvasContext = createContext<PipelineCanvasContextShape>(DEFAULT_CONTEXT);
PipelineCanvasContext.displayName = "PipelineCanvasContext";

interface PipelineCanvasProviderProps {
  initialState?: Partial<PipelineCanvasState>;
}

const PipelineCanvasProvider = ({
  children,
  initialState,
}: PropsWithChildren<PipelineCanvasProviderProps>) => {
  const [state, dispatch] = useReducer(pipelineCanvasReducer, {
    ...DEFAULT_STATE,
    ...initialState,
  });

  return (
    <PipelineCanvasContext.Provider value={{ state, dispatch }}>
      {children}
    </PipelineCanvasContext.Provider>
  );
};

export default PipelineCanvasProvider;
