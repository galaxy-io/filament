import { useCallback } from "react";

import {
  addEdge,
  type Connection,
  type OnConnect,
  useEdgesState,
  useNodesState,
} from "@xyflow/react";

import type { PipelineEdge, PipelineNode } from "@/pages/pipelines/canvas/types";

interface UsePipelineCanvasOptions {
  initialNodes?: PipelineNode[];
  initialEdges?: PipelineEdge[];
}

const usePipelineCanvas = (options?: UsePipelineCanvasOptions) => {
  const [nodes, setNodes, onNodesChange] = useNodesState<PipelineNode>(options?.initialNodes ?? []);

  const [edges, setEdges, onEdgesChange] = useEdgesState<PipelineEdge>(options?.initialEdges ?? []);

  const onConnect: OnConnect = useCallback(
    (connection: Connection) => {
      setEdges((eds) => addEdge(connection, eds));
    },
    [setEdges],
  );

  const addNode = useCallback(
    (node: PipelineNode) => {
      setNodes((nds) => [...nds, node]);
    },
    [setNodes],
  );

  const removeNode = useCallback(
    (nodeId: string) => {
      setNodes((nds) => nds.filter((node) => node.id !== nodeId));
      setEdges((eds) => eds.filter((edge) => edge.source !== nodeId && edge.target !== nodeId));
    },
    [setNodes, setEdges],
  );

  const removeEdge = useCallback(
    (edgeId: string) => {
      setEdges((eds) => eds.filter((edge) => edge.id !== edgeId));
    },
    [setEdges],
  );

  return {
    nodes,
    edges,
    onNodesChange,
    onEdgesChange,
    onConnect,
    addNode,
    removeNode,
    removeEdge,
  };
};

export default usePipelineCanvas;
