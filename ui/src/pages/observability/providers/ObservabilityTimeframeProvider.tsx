import { createContext, type PropsWithChildren, useContext, useMemo, useState } from "react";

import { ObservabilityTimeframe } from "@/pages/observability/types";

interface ObservabilityTimeframeContextShape {
  timeframe: ObservabilityTimeframe;
  setTimeframe: (timeframe: ObservabilityTimeframe) => void;
}

const ObservabilityTimeframeContext = createContext<ObservabilityTimeframeContextShape | null>(
  null,
);
ObservabilityTimeframeContext.displayName = "ObservabilityTimeframeContext";

export const useObservabilityTimeframe = () => {
  const context = useContext(ObservabilityTimeframeContext);
  if (!context) {
    throw new Error("useObservabilityTimeframe must be used within ObservabilityTimeframeProvider");
  }
  return context;
};

const ObservabilityTimeframeProvider = ({ children }: PropsWithChildren) => {
  const [timeframe, setTimeframe] = useState(ObservabilityTimeframe.TWENTY_FOUR_HOURS);

  const value = useMemo(() => ({ timeframe, setTimeframe }), [timeframe]);

  return (
    <ObservabilityTimeframeContext.Provider value={value}>
      {children}
    </ObservabilityTimeframeContext.Provider>
  );
};

export default ObservabilityTimeframeProvider;
