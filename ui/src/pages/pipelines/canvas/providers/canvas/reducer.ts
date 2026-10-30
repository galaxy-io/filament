import {
  type Connection as CanvasConnection,
  addEdge as xyflowAddEdge,
  applyEdgeChanges as xyflowApplyEdgeChanges,
  applyNodeChanges as xyflowApplyNodeChanges,
} from "@xyflow/react";

import { ReadMode } from "@/gen/ingestion/v1/common_pb";

import { PIPELINE_CANVAS_EDGE_TYPE } from "@/pages/pipelines/canvas/constants";
import {
  canAddSourceNode,
  canConnectEdge,
  getAutoConnections,
} from "@/pages/pipelines/canvas/graph/rules";
import {
  getCanvasEdgeConfig,
  getCanvasEdgeKey,
  getProtoEdgeKey,
} from "@/pages/pipelines/canvas/graph/serialize";
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
  type SetEdgeConfigAction,
  type SetNodeConfigAction,
  type SetNodesAction,
  type SetRouteWriteModeAction,
  type SettleGraphAction,
} from "@/pages/pipelines/canvas/providers/canvas/actions";
import type { PipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/types";
import { type CanvasEdge, PipelineCanvasNodeType } from "@/pages/pipelines/canvas/types";
import { isConnectionNode } from "@/pages/pipelines/canvas/utils";

function loadGraph(state: PipelineCanvasState, action: LoadGraphAction): PipelineCanvasState {
  return {
    ...state,
    nodes: action.payload.nodes,
    edges: action.payload.edges,
    activeMode: null,
  };
}

function addEdges(edges: CanvasEdge[], connections: CanvasConnection[]): CanvasEdge[] {
  return connections.reduce((nextEdges, connection) => {
    if (!canConnectEdge(connection, nextEdges)) return nextEdges;
    const routeWriteMode = nextEdges.find(
      (edge) => edge.source === connection.source && edge.target === connection.target,
    )?.data?.writeMode;
    return xyflowAddEdge(
      {
        ...connection,
        type: PIPELINE_CANVAS_EDGE_TYPE,
        ...(routeWriteMode !== undefined && {
          data: { readMode: ReadMode.UNSPECIFIED, writeMode: routeWriteMode, cursors: [] },
        }),
      },
      nextEdges,
    );
  }, edges);
}

function addNode(state: PipelineCanvasState, action: AddNodeAction): PipelineCanvasState {
  const { node, resources } = action.payload;
  if (node.type === PipelineCanvasNodeType.SOURCE && !canAddSourceNode(state.nodes)) {
    return state;
  }

  return {
    ...state,
    nodes: [...state.nodes, node],
    edges: addEdges(state.edges, getAutoConnections(node, state.nodes, resources)),
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

function settleGraph(state: PipelineCanvasState, action: SettleGraphAction): PipelineCanvasState {
  const { version, savedNodeConfigs, savedEdgeData } = action.payload;
  const versionNodesById = new Map((version.graph?.nodes ?? []).map((node) => [node.id, node]));
  const versionEdgesByKey = new Map(
    (version.graph?.edges ?? []).map((edge) => [getProtoEdgeKey(edge), edge]),
  );
  return {
    ...state,
    nodes: state.nodes.map((node) => {
      const savedNode = versionNodesById.get(node.id);
      return isConnectionNode(node) &&
        node.data.config &&
        savedNodeConfigs.has(node.data.config) &&
        savedNode
        ? { ...node, data: { ...node.data, config: savedNode.config } }
        : node;
    }),
    edges: state.edges.map((edge) => {
      const savedEdge = versionEdgesByKey.get(getCanvasEdgeKey(edge));
      return edge.data && savedEdgeData.has(edge.data) && savedEdge
        ? { ...edge, data: getCanvasEdgeConfig({}, savedEdge) }
        : edge;
    }),
  };
}

function connect(state: PipelineCanvasState, action: ConnectAction): PipelineCanvasState {
  return { ...state, edges: addEdges(state.edges, action.payload) };
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

function setEdgeConfig(
  state: PipelineCanvasState,
  action: SetEdgeConfigAction,
): PipelineCanvasState {
  return {
    ...state,
    edges: state.edges.map((edge) =>
      edge.id === action.payload.edgeId
        ? { ...edge, data: { ...edge.data, ...action.payload.data } }
        : edge,
    ),
  };
}

function setRouteWriteMode(
  state: PipelineCanvasState,
  action: SetRouteWriteModeAction,
): PipelineCanvasState {
  return {
    ...state,
    edges: state.edges.map((edge) =>
      edge.source === action.payload.source && edge.target === action.payload.target
        ? {
            ...edge,
            data: {
              ...edge.data,
              readMode: edge.data?.readMode ?? ReadMode.UNSPECIFIED,
              writeMode: action.payload.writeMode,
              cursors: edge.data?.cursors ?? [],
              transform: edge.data?.transform,
            },
          }
        : edge,
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
    case PipelineCanvasActionType.SET_NODE_CONFIG:
      return setNodeConfig(state, action);
    case PipelineCanvasActionType.SET_EDGE_CONFIG:
      return setEdgeConfig(state, action);
    case PipelineCanvasActionType.SET_ROUTE_WRITE_MODE:
      return setRouteWriteMode(state, action);
    case PipelineCanvasActionType.SETTLE_GRAPH:
      return settleGraph(state, action);
  }
};

export default pipelineCanvasReducer;
