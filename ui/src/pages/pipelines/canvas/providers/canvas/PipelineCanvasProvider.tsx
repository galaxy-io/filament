import {
  createContext,
  type Dispatch,
  type PropsWithChildren,
  useContext,
  useMemo,
  useReducer,
  useState,
} from "react";

import type { JsonValue } from "@bufbuild/protobuf";
import type { Connection, EdgeChange, NodeChange } from "@xyflow/react";

import type { IngestionType } from "@/gen/ingestion/v1/common_pb";

import { pokePipelineCanvasValidation } from "@/pages/pipelines/canvas/graph/validationClock";
import {
  type PipelineCanvasAction,
  PipelineCanvasActionType,
} from "@/pages/pipelines/canvas/providers/canvas/actions";
import pipelineCanvasReducer from "@/pages/pipelines/canvas/providers/canvas/reducer";
import type {
  PipelineCanvasEditMode,
  PipelineCanvasGraph,
  PipelineCanvasInteractionMode,
  PipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/types";
import { createInitialPipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/utils";
import type { CanvasEdge, CanvasNode } from "@/pages/pipelines/canvas/types";

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

const usePipelineCanvasDispatch = () => {
  const dispatch = useContext(PipelineCanvasDispatchContext);
  if (!dispatch) {
    throw new Error("usePipelineCanvasDispatch must be used within PipelineCanvasProvider");
  }
  return dispatch;
};

export const usePipelineCanvasReadOnly = () => useContext(PipelineCanvasReadOnlyContext);

export const usePipelineCanvasActions = () => {
  const dispatch = usePipelineCanvasDispatch();

  return useMemo(() => {
    // Only dispatches that change the wire graph restart the validation
    // debounce; node drags and selection churn would otherwise poke it on
    // every mousemove.
    const dispatchAndValidate = (action: PipelineCanvasAction) => {
      dispatch(action);
      pokePipelineCanvasValidation();
    };

    return {
      loadGraph: (graph: PipelineCanvasGraph) =>
        dispatchAndValidate({ type: PipelineCanvasActionType.LOAD_GRAPH, payload: graph }),
      addNode: (node: CanvasNode) =>
        dispatchAndValidate({ type: PipelineCanvasActionType.ADD_NODE, payload: node }),
      removeNode: (nodeId: string) =>
        dispatchAndValidate({
          type: PipelineCanvasActionType.REMOVE_NODE,
          payload: nodeId,
        }),
      setNodes: (nodes: CanvasNode[]) =>
        dispatch({ type: PipelineCanvasActionType.SET_NODES, payload: nodes }),
      applyNodeChanges: (changes: NodeChange<CanvasNode>[]) =>
        dispatch({
          type: PipelineCanvasActionType.APPLY_NODE_CHANGES,
          payload: changes,
        }),
      applyEdgeChanges: (changes: EdgeChange<CanvasEdge>[]) =>
        dispatchAndValidate({
          type: PipelineCanvasActionType.APPLY_EDGE_CHANGES,
          payload: changes,
        }),
      connect: (connection: Connection) =>
        dispatchAndValidate({
          type: PipelineCanvasActionType.CONNECT,
          payload: connection,
        }),
      setActiveMode: (mode: PipelineCanvasEditMode | null) =>
        dispatch({
          type: PipelineCanvasActionType.SET_ACTIVE_MODE,
          payload: mode,
        }),
      setInteractionMode: (mode: PipelineCanvasInteractionMode) =>
        dispatch({
          type: PipelineCanvasActionType.SET_INTERACTION_MODE,
          payload: mode,
        }),
      setNodeConfig: (nodeId: string, config: Record<string, JsonValue>) =>
        dispatchAndValidate({
          type: PipelineCanvasActionType.SET_NODE_CONFIG,
          payload: { nodeId, config },
        }),
      setEdgeIngestionType: (edgeId: string, ingestionType: IngestionType) =>
        dispatchAndValidate({
          type: PipelineCanvasActionType.SET_EDGE_INGESTION_TYPE,
          payload: { edgeId, ingestionType },
        }),
      setEdgeCursor: (edgeId: string, resource: string, field: string, lookbackSeconds: number) =>
        dispatchAndValidate({
          type: PipelineCanvasActionType.SET_EDGE_CURSOR,
          payload: { edgeId, resource, field, lookbackSeconds },
        }),
    };
  }, [dispatch]);
};

interface PipelineCanvasProviderProps {
  graph: PipelineCanvasGraph;
  graphKey: string;
  isReadOnly?: boolean;
}

const PipelineCanvasProvider = ({
  graph,
  graphKey,
  isReadOnly = false,
  children,
}: PropsWithChildren<PipelineCanvasProviderProps>) => {
  const [state, dispatch] = useReducer(
    pipelineCanvasReducer,
    createInitialPipelineCanvasState(graph),
  );

  const [previousGraphKey, setPreviousGraphKey] = useState(graphKey);
  if (previousGraphKey !== graphKey) {
    setPreviousGraphKey(graphKey);
    dispatch({ type: PipelineCanvasActionType.LOAD_GRAPH, payload: graph });
  }

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
