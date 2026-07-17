import { addEdge, applyEdgeChanges, applyNodeChanges } from "@xyflow/react";

import {
  type PipelineCanvasAction,
  PipelineCanvasActionType,
} from "@/pages/pipelines/canvas/actions";
import type { PipelineCanvasState } from "@/pages/pipelines/canvas/types";

const pipelineCanvasReducer = (
  state: PipelineCanvasState,
  action: PipelineCanvasAction,
): PipelineCanvasState => {
  switch (action.type) {
    case PipelineCanvasActionType.ADD_NODE:
      return { ...state, nodes: [...state.nodes, action.payload] };

    case PipelineCanvasActionType.REMOVE_NODE:
      return {
        ...state,
        nodes: state.nodes.filter((node) => node.id !== action.payload),
        edges: state.edges.filter(
          (edge) => edge.source !== action.payload && edge.target !== action.payload,
        ),
      };

    case PipelineCanvasActionType.SET_NODES:
      return { ...state, nodes: action.payload };

    case PipelineCanvasActionType.APPLY_NODE_CHANGES:
      return { ...state, nodes: applyNodeChanges(action.payload, state.nodes) };

    case PipelineCanvasActionType.ADD_EDGE:
      return { ...state, edges: [...state.edges, action.payload] };

    case PipelineCanvasActionType.REMOVE_EDGE:
      return {
        ...state,
        edges: state.edges.filter((edge) => edge.id !== action.payload),
      };

    case PipelineCanvasActionType.SET_EDGES:
      return { ...state, edges: action.payload };

    case PipelineCanvasActionType.APPLY_EDGE_CHANGES:
      return { ...state, edges: applyEdgeChanges(action.payload, state.edges) };

    case PipelineCanvasActionType.CONNECT:
      return { ...state, edges: addEdge(action.payload, state.edges) };

    case PipelineCanvasActionType.SET_ACTIVE_MODE:
      return { ...state, activeMode: action.payload };

    default:
      return state;
  }
};

export default pipelineCanvasReducer;
