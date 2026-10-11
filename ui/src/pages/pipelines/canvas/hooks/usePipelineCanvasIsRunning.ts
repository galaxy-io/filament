import { usePipelineParams } from "@/module/hooks";

import { useListActivePipelineRunsQuery } from "@/api/queries/runs";

export const usePipelineCanvasIsRunning = () => {
  const { id } = usePipelineParams();
  const { data } = useListActivePipelineRunsQuery(id);
  return (data?.runs.length ?? 0) > 0;
};
