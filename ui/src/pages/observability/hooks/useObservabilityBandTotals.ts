import type { Metric } from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_BAND_TOTAL_METRICS } from "@/pages/observability/components/band/constants";
import { useObservabilityAggregateQuery } from "@/pages/observability/hooks/useObservabilityAggregateQuery";
import { useObservabilityMetricInputs } from "@/pages/observability/hooks/useObservabilityMetricInputs";

export const useObservabilityBandTotals = () => {
  const { totalsInput } = useObservabilityMetricInputs();
  const { data, isLoading } = useObservabilityAggregateQuery(totalsInput);
  const values = data?.rows[0]?.values ?? [];

  const getTotal = (metric: Metric) =>
    values[OBSERVABILITY_BAND_TOTAL_METRICS.indexOf(metric)] ?? 0;

  return { getTotal, isLoading };
};
