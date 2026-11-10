import { useMemo } from "react";

import { formatRelativeTime } from "@galaxy-io/dls/utils/format";

import type { ObservabilityBandListItem } from "@/pages/observability/components/band/types";
import {
  filterObservabilityBandUpcomingRuns,
  getObservabilityBandUpcomingBounds,
  mapObservabilityBandUpcomingGroups,
} from "@/pages/observability/components/band/utils";
import { OBSERVABILITY_RUNS_SCHEDULED_INPUT } from "@/pages/observability/components/runs/constants";
import { useObservabilityMetricInputs } from "@/pages/observability/hooks/useObservabilityMetricInputs";
import { useObservabilityPipelines } from "@/pages/observability/hooks/useObservabilityPipelines";

import { useObservabilitySearch } from "@/module/hooks";

import { useListRunsQuery } from "@/api/queries/runs";

import { formatTimestamp } from "@/utils/format";

export const useObservabilityBandUpcoming = () => {
  const { timeframe } = useObservabilitySearch();
  const { bucketStartMs } = useObservabilityMetricInputs();
  const { pipelinesById } = useObservabilityPipelines();
  const { data, isLoading, isFetching } = useListRunsQuery({
    input: OBSERVABILITY_RUNS_SCHEDULED_INPUT,
  });

  const upcoming = useMemo(() => {
    const bounds = getObservabilityBandUpcomingBounds(timeframe, bucketStartMs);
    const runs = filterObservabilityBandUpcomingRuns(data?.runs ?? [], bounds);
    const nextUp = new Map<string, ObservabilityBandListItem>();
    for (const run of runs) {
      const pipeline = pipelinesById.get(run.pipelineId);
      if (pipeline && !nextUp.has(pipeline.id)) {
        nextUp.set(pipeline.id, {
          pipeline,
          value: formatRelativeTime(run.scheduledAt),
          tooltip: formatTimestamp(run.scheduledAt),
        });
      }
    }
    return {
      count: runs.length,
      groups: mapObservabilityBandUpcomingGroups(runs, bounds),
      nextUp: [...nextUp.values()],
    };
  }, [data, timeframe, bucketStartMs, pipelinesById]);

  return { ...upcoming, isLoading, isFetching };
};
