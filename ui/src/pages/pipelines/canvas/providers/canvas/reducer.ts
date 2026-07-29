import {
  addEdge as xyflowAddEdge,
  applyEdgeChanges as xyflowApplyEdgeChanges,
  applyNodeChanges as xyflowApplyNodeChanges,
} from "@xyflow/react";

import { PIPELINE_CANVAS_EDGE_TYPE } from "@/pages/pipelines/canvas/constants";
import { canAddSourceNode, getAutoConnections } from "@/pages/pipelines/canvas/graph/rules";
import {
  type AddNodeAction,
  type ApplyEdgeChangesAction,
  type ApplyNodeChangesAction,
  type ConnectAction,
  type LoadGraphAction,
  type PipelineCanvasAction,
  PipelineCanvasActionType,
  type RemoveNodeAction,
  type SetActiveModeAction,
  type SetInteractionModeAction,
  type SetNodeConfigAction,
  type SetNodesAction,
} from "@/pages/pipelines/canvas/providers/canvas/actions";
import {
  PipelineCanvasInteractionMode,
  type PipelineCanvasState,
} from "@/pages/pipelines/canvas/providers/canvas/types";
import { isConnectionNode, PipelineCanvasNodeType } from "@/pages/pipelines/canvas/types";

function loadGraph(state: PipelineCanvasState, action: LoadGraphAction): PipelineCanvasState {
  return {
    ...state,
    nodes: action.payload.nodes,
    edges: action.payload.edges,
    activeMode: null,
    interactionMode: PipelineCanvasInteractionMode.GRAB,
  };
}

function addNode(state: PipelineCanvasState, action: AddNodeAction): PipelineCanvasState {
  const node = action.payload;
  // Silent backstop against graph corruption: every dispatch site already
  // presents this rule via canAddSourceNode, so rejection needs no feedback.
  if (node.type === PipelineCanvasNodeType.SOURCE && !canAddSourceNode(state.nodes)) {
    return state;
  }

  const edges = getAutoConnections(node, state.nodes).reduce(
    (nextEdges, connection) =>
      xyflowAddEdge({ ...connection, type: PIPELINE_CANVAS_EDGE_TYPE }, nextEdges),
    state.edges,
  );

  return { ...state, nodes: [...state.nodes, node], edges };
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

function setNodeConfig(
  state: PipelineCanvasState,
  action: SetNodeConfigAction,
): PipelineCanvasState {
  return {
    ...state,
    nodes: state.nodes.map((node) =>
      node.id === action.payload.nodeId && isConnectionNode(node)
        ? { ...node, data: { ...node.data, config: action.payload.config } }
        : node,
    ),
  };
}

const pipelineCanvasReducer = (
  state: PipelineCanvasState,
  action: PipelineCanvasAction,
): PipelineCanvasState => {
  switch (action.type) {
    case PipelineCanvasActionType.LOAD_GRAPH:
      return loadGraph(state, action);
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
    case PipelineCanvasActionType.SET_NODE_CONFIG:
      return setNodeConfig(state, action);
  }
};

export default pipelineCanvasReducer;
