import { useCallback } from "react";

import { useFilamentNavigate, usePipelineParams } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";
import type { FilamentLayoutSearch, PipelineCanvasSearch, PipelineSearch } from "@/module/schemas";

type PipelineCanvasNavigateSearch = PipelineCanvasSearch & PipelineSearch;

export const usePipelineCanvasNavigate = () => {
  const navigate = useFilamentNavigate();
  const { id } = usePipelineParams();

  return useCallback(
    (patch: PipelineCanvasNavigateSearch) =>
      void navigate({
        to: FilamentPath.PIPELINE_CANVAS,
        params: { id },
        search: (prev: FilamentLayoutSearch & PipelineCanvasNavigateSearch) => ({
          ...prev,
          ...patch,
        }),
      }),
    [navigate, id],
  );
};
