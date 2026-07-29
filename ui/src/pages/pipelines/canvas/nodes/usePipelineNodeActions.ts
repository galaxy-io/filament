import { useState } from "react";

import type { JsonValue } from "@bufbuild/protobuf";

import { PipelineCanvasActionType } from "@/pages/pipelines/canvas/providers/canvas/actions";
import { usePipelineCanvasDispatch } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";

export const usePipelineNodeActions = (nodeId: string) => {
  const dispatch = usePipelineCanvasDispatch();
  const [isConfigOpen, setIsConfigOpen] = useState(false);

  const toggleConfigOpen = () => setIsConfigOpen((open) => !open);

  const removeNode = () => {
    dispatch({ type: PipelineCanvasActionType.REMOVE_NODE, payload: nodeId });
  };

  const setNodeConfig = (config: Record<string, JsonValue>) => {
    dispatch({ type: PipelineCanvasActionType.SET_NODE_CONFIG, payload: { nodeId, config } });
  };

  return { isConfigOpen, toggleConfigOpen, removeNode, setNodeConfig };
};
