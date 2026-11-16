import { useMemo } from "react";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import { PIPELINE_CANVAS_NO_RUNS } from "@/pages/pipelines/canvas/constants";
import { usePipelineExecutionLane } from "@/pages/pipelines/hooks/usePipelineExecutionLane";

import { usePipelineParams } from "@/module/hooks";

import { useListActivePipelineRunsQuery } from "@/api/queries/runs";

export const usePipelineCanvasActiveRuns = (): RunInfo[] => {
  const { id } = usePipelineParams();
  const { data } = useListActivePipelineRunsQuery(id);
  const { isRunInLane } = usePipelineExecutionLane();
  const runs = data?.runs ?? PIPELINE_CANVAS_NO_RUNS;
  return useMemo(() => runs.filter(isRunInLane), [runs, isRunInLane]);
};
