import { useMemo } from "react";

import { hasPipelineGraphChanges } from "@/pages/pipelines/canvas/graph/diff";
import { usePipelineCanvasState } from "@/pages/pipelines/canvas/providers/canvas/PipelineCanvasProvider";
import { usePipelinePreviewVersion } from "@/pages/pipelines/hooks/usePipelinePreviewVersion";

import { usePipelineParams } from "@/module/hooks";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";

export const usePipelineHasUnsavedChanges = () => {
  const { id } = usePipelineParams();
  const state = usePipelineCanvasState();
  const isPreview = usePipelinePreviewVersion() !== undefined;

  const { data } = useSuspenseGetPipelineQuery({
    input: createGetPipelineInput(id),
  });
  const currentVersion = data.pipeline?.currentVersion;

  return useMemo(
    () =>
      !isPreview &&
      hasPipelineGraphChanges({ nodes: state.nodes, edges: state.edges }, currentVersion),
    [isPreview, state.nodes, state.edges, currentVersion],
  );
};
