import { useCallback, useSyncExternalStore } from "react";

import { OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP } from "@/pages/observability/constants";
import type { ObservabilityTimeframe } from "@/pages/observability/types";
import { getBucketOffset, getBucketStart } from "@/pages/observability/utils";

export const useObservabilityBucketStart = (timeframe: ObservabilityTimeframe) => {
  const { granularity } = OBSERVABILITY_TIMEFRAME_TO_QUERY_MAP[timeframe];

  const subscribe = useCallback(
    (onBucketChange: () => void) => {
      let timeout: ReturnType<typeof setTimeout> | undefined;
      const scheduleNextBucket = () => {
        const nextBucketStartMs = getBucketOffset(
          granularity,
          getBucketStart(granularity, new Date()),
          1,
        );
        timeout = setTimeout(() => {
          onBucketChange();
          scheduleNextBucket();
        }, nextBucketStartMs - Date.now());
      };
      scheduleNextBucket();
      return () => clearTimeout(timeout);
    },
    [granularity],
  );

  const getSnapshot = useCallback(
    () => getBucketStart(granularity, new Date()).getTime(),
    [granularity],
  );

  return useSyncExternalStore(subscribe, getSnapshot);
};
