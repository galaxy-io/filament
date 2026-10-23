import { type FC, useState } from "react";

import { ArrowsClockwiseIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import ObservabilityRefreshIntervalSelect from "@/pages/observability/components/ObservabilityRefreshIntervalSelect";
import ObservabilityTimeframeSwitcher from "@/pages/observability/components/ObservabilityTimeframeSwitcher";
import type {
  ObservabilityRefreshInterval,
  ObservabilityTimeframe,
} from "@/pages/observability/types";

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import type { ObservabilitySearch } from "@/module/schemas";

import { useRefreshObservabilityQueries } from "@/api/queries/metrics";

const ObservabilityPageActions: FC = () => {
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const refresh = useRefreshObservabilityQueries();
  const [isRefreshing, setIsRefreshing] = useState(false);
  const { timeframe, refreshInterval } = useObservabilitySearch();

  const handleTimeframeChange = (timeframe: ObservabilityTimeframe) => {
    void updateSearch((prev) => ({
      ...prev,
      timeframe,
      runsBucket: undefined,
      runsStatus: undefined,
    }));
  };

  const handleRefreshIntervalChange = (refreshInterval: ObservabilityRefreshInterval) => {
    void updateSearch((prev) => ({ ...prev, refreshInterval }));
  };

  const handleRefresh = async () => {
    setIsRefreshing(true);
    await refresh();
    setIsRefreshing(false);
  };

  return (
    <>
      <ObservabilityTimeframeSwitcher value={timeframe} onChange={handleTimeframeChange} />
      <ObservabilityRefreshIntervalSelect
        value={refreshInterval}
        onChange={handleRefreshIntervalChange}
      />
      <Button
        icon={ArrowsClockwiseIcon}
        variant={ButtonVariant.SECONDARY}
        ariaLabel="Refresh"
        tooltip="Refresh"
        isLoading={isRefreshing}
        onClick={handleRefresh}
      />
    </>
  );
};

export default ObservabilityPageActions;
