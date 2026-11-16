import { useState } from "react";

import type { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import { PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_RUNS } from "@/pages/pipelines/canvas/panel/activity/constants";
import { usePipelineExecutionLane } from "@/pages/pipelines/hooks/usePipelineExecutionLane";

import { usePipelineParams } from "@/module/hooks";

import { useListActivePipelineRunsQuery } from "@/api/queries/runs";

export const usePipelineCanvasPanelActivityRunIds = () => {
  const { id } = usePipelineParams();

  const { data: activeRunsData } = useListActivePipelineRunsQuery(id);
  const { lane, isRunInLane } = usePipelineExecutionLane();

  const [state, setState] = useState<{ lane: ExecutionMode; runIds: RunInfo["id"][] }>({
    lane,
    runIds: [],
  });
  const previousRunIds = state.lane === lane ? state.runIds : [];
  const mergedRunIds = [
    ...new Set([
      ...previousRunIds,
      ...(activeRunsData?.runs ?? []).filter(isRunInLane).map((run) => run.id),
    ]),
  ].slice(-PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_RUNS);
  if (state.lane !== lane || mergedRunIds.join("|") !== state.runIds.join("|")) {
    setState({ lane, runIds: mergedRunIds });
  }

  return mergedRunIds;
};
