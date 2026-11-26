import { getCanvasEdgeKey, getProtoEdgeKey } from "@/pages/pipelines/canvas/graph/serialize";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";

import { usePipelineParams } from "@/module/hooks";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

export const usePipelineCanvasEdgeSaved = (edge: CanvasEdge): boolean => {
  const { id } = usePipelineParams();
  const { data } = useSuspenseGetPipelineQuery({ input: createGetPipelineInput(id) });
  const edgeKey = getCanvasEdgeKey(edge);
  return (data.pipeline?.currentVersion?.graph?.edges ?? []).some(
    (saved) => getProtoEdgeKey(saved) === edgeKey,
  );
};
