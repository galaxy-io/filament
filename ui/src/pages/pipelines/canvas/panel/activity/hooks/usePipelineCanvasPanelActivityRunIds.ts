import { useState } from "react";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import { PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_RUNS } from "@/pages/pipelines/canvas/panel/activity/constants";

import { usePipelineParams } from "@/module/hooks";

import { useListActivePipelineRunsQuery } from "@/api/queries/runs";

export const usePipelineCanvasPanelActivityRunIds = () => {
  const { id } = usePipelineParams();

  const { data: activeRunsData } = useListActivePipelineRunsQuery(id);

  const [runIds, setRunIds] = useState<RunInfo["id"][]>([]);
  const mergedRunIds = [
    ...new Set([...runIds, ...(activeRunsData?.runs ?? []).map((run) => run.id)]),
  ].slice(-PIPELINE_CANVAS_PANEL_ACTIVITY_MAX_RUNS);
  if (mergedRunIds.join("|") !== runIds.join("|")) {
    setRunIds(mergedRunIds);
  }

  return runIds;
};
