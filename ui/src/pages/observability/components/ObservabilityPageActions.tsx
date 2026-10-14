import type { FC } from "react";

import { ArrowsClockwiseIcon } from "@phosphor-icons/react";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";

import ObservabilityTimeframeSwitcher from "@/pages/observability/components/ObservabilityTimeframeSwitcher";
import type { ObservabilityTimeframe } from "@/pages/observability/types";

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import type { ObservabilitySearch } from "@/module/schemas";

import { useRefreshObservabilityQueries } from "@/api/queries/metrics";

const ObservabilityPageActions: FC = () => {
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const refresh = useRefreshObservabilityQueries();
  const { timeframe } = useObservabilitySearch();

  const handleTimeframeChange = (timeframe: ObservabilityTimeframe) => {
    void updateSearch((prev) => ({
      ...prev,
      timeframe,
      runsBucket: undefined,
      runsStatus: undefined,
    }));
  };

  return (
    <>
      <ObservabilityTimeframeSwitcher value={timeframe} onChange={handleTimeframeChange} />
      <Button
        icon={ArrowsClockwiseIcon}
        variant={ButtonVariant.SECONDARY}
        ariaLabel="Refresh"
        tooltip="Refresh"
        onClick={refresh}
      />
    </>
  );
};

export default ObservabilityPageActions;
