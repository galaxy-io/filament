import {
  addEdge as xyflowAddEdge,
  applyEdgeChanges as xyflowApplyEdgeChanges,
  applyNodeChanges as xyflowApplyNodeChanges,
} from "@xyflow/react";

import { PIPELINE_CANVAS_EDGE_TYPE } from "@/pages/pipelines/canvas/constants";
import {
  type AddNodeAction,
  type ApplyEdgeChangesAction,
  type ApplyNodeChangesAction,
  type ConnectAction,
  type PipelineCanvasAction,
  PipelineCanvasActionType,
  type RemoveNodeAction,
  type SetActiveModeAction,
  type SetInteractionModeAction,
  type SetNodesAction,
} from "@/pages/pipelines/canvas/providers/canvas/actions";
import type { PipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/types";

function addNode(state: PipelineCanvasState, action: AddNodeAction): PipelineCanvasState {
  return {
    ...state,
    nodes: [...state.nodes, action.payload],
  };
}

function removeNode(state: PipelineCanvasState, action: RemoveNodeAction): PipelineCanvasState {
  return {
    ...state,
    nodes: state.nodes.filter((node) => node.id !== action.payload),
    edges: state.edges.filter(
      (edge) => edge.source !== action.payload && edge.target !== action.payload,
    ),
  };
}

function setNodes(state: PipelineCanvasState, action: SetNodesAction): PipelineCanvasState {
  return {
    ...state,
    nodes: action.payload,
  };
}

function applyNodeChanges(
  state: PipelineCanvasState,
  action: ApplyNodeChangesAction,
): PipelineCanvasState {
  return {
    ...state,
    nodes: xyflowApplyNodeChanges(action.payload, state.nodes),
  };
}

function applyEdgeChanges(
  state: PipelineCanvasState,
  action: ApplyEdgeChangesAction,
): PipelineCanvasState {
  return {
    ...state,
    edges: xyflowApplyEdgeChanges(action.payload, state.edges),
  };
}

function connect(state: PipelineCanvasState, action: ConnectAction): PipelineCanvasState {
  return {
    ...state,
    edges: xyflowAddEdge({ ...action.payload, type: PIPELINE_CANVAS_EDGE_TYPE }, state.edges),
  };
}

function setActiveMode(
  state: PipelineCanvasState,
  action: SetActiveModeAction,
): PipelineCanvasState {
  return {
    ...state,
    activeMode: action.payload,
  };
}

function setInteractionMode(
  state: PipelineCanvasState,
  action: SetInteractionModeAction,
): PipelineCanvasState {
  return {
    ...state,
    interactionMode: action.payload,
  };
}

const pipelineCanvasReducer = (
  state: PipelineCanvasState,
  action: PipelineCanvasAction,
): PipelineCanvasState => {
  switch (action.type) {
    case PipelineCanvasActionType.ADD_NODE:
      return addNode(state, action);
    case PipelineCanvasActionType.REMOVE_NODE:
      return removeNode(state, action);
    case PipelineCanvasActionType.SET_NODES:
      return setNodes(state, action);
    case PipelineCanvasActionType.APPLY_NODE_CHANGES:
      return applyNodeChanges(state, action);
    case PipelineCanvasActionType.APPLY_EDGE_CHANGES:
      return applyEdgeChanges(state, action);
    case PipelineCanvasActionType.CONNECT:
      return connect(state, action);
    case PipelineCanvasActionType.SET_ACTIVE_MODE:
      return setActiveMode(state, action);
    case PipelineCanvasActionType.SET_INTERACTION_MODE:
      return setInteractionMode(state, action);
  }
};

export default pipelineCanvasReducer;
