import { useEffect, useState } from "react";

import { PIPELINE_RUN_DURATION_TICK_MS } from "@/components/runs/constants";

export const usePipelineRunNow = (isTicking: boolean) => {
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!isTicking) return;
    const interval = setInterval(() => setNow(Date.now()), PIPELINE_RUN_DURATION_TICK_MS);
    return () => clearInterval(interval);
  }, [isTicking]);

  return now;
};
