import { useMemo } from "react";

import {
  OBSERVABILITY_PIPELINE_HEALTH_TO_RANK_MAP,
  OBSERVABILITY_PIPELINES_INPUT,
} from "@/pages/observability/constants";
import { ObservabilityPipelineHealth } from "@/pages/observability/types";
import { getObservabilityPipelineHealth } from "@/pages/observability/utils";

import { useSuspenseListPipelinesQuery } from "@/api/queries/pipelines";

export const useObservabilityPipelines = () => {
  const { data } = useSuspenseListPipelinesQuery({ input: OBSERVABILITY_PIPELINES_INPUT });

  return useMemo(() => {
    const entries = data.pipelines
      .map((pipeline) => ({ pipeline, health: getObservabilityPipelineHealth(pipeline) }))
      .sort(
        (a, b) =>
          OBSERVABILITY_PIPELINE_HEALTH_TO_RANK_MAP[a.health] -
            OBSERVABILITY_PIPELINE_HEALTH_TO_RANK_MAP[b.health] ||
          Number((b.pipeline.lastRun?.requestedAt ?? 0n) - (a.pipeline.lastRun?.requestedAt ?? 0n)),
      );
    return {
      entries,
      failing: entries.filter(({ health }) => health === ObservabilityPipelineHealth.FAILING),
      pipelinesById: new Map(data.pipelines.map((pipeline) => [pipeline.id, pipeline])),
    };
  }, [data]);
};
