import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { usePipelineCanvasEdgeSaved } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasEdgeSaved";
import type { CanvasEdge } from "@/pages/pipelines/canvas/types";

import { usePipelineParams } from "@/module/hooks";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

export const usePipelineCanvasEdgeLocked = (edge: CanvasEdge): boolean => {
  const { id } = usePipelineParams();
  const { data } = useSuspenseGetPipelineQuery({ input: createGetPipelineInput(id) });
  const isSavedEdge = usePipelineCanvasEdgeSaved(edge);
  const { currentVersion, lastRun } = data.pipeline ?? {};
  return (
    isSavedEdge &&
    lastRun?.status === RunStatus.COMPLETED &&
    lastRun.pipelineVersionId === currentVersion?.id
  );
};
