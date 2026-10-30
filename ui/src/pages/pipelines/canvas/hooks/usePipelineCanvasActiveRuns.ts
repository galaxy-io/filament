import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import { PIPELINE_CANVAS_NO_RUNS } from "@/pages/pipelines/canvas/constants";

import { usePipelineParams } from "@/module/hooks";

import { useListActivePipelineRunsQuery } from "@/api/queries/runs";

export const usePipelineCanvasActiveRuns = (): RunInfo[] => {
  const { id } = usePipelineParams();
  const { data } = useListActivePipelineRunsQuery(id);
  return data?.runs ?? PIPELINE_CANVAS_NO_RUNS;
};
