import type { FC } from "react";

import { useTransport } from "@connectrpc/connect-query";
import { ArrowsClockwiseIcon } from "@phosphor-icons/react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Topbar from "@galaxy-io/dls/navigation/Topbar";

import ObservabilityTimeframeSwitcher from "@/pages/observability/components/ObservabilityTimeframeSwitcher";
import { ObservabilityTimeframe } from "@/pages/observability/types";

import { createListConnectionsQueryKey } from "@/api/queries/connections";
import { createQueryAggregateQueryKey, createQueryTimeseriesQueryKey } from "@/api/queries/metrics";
import { createListPipelinesQueryKey } from "@/api/queries/pipelines";
import { createListRunsQueryKey } from "@/api/queries/runs";

const ObservabilityToolbar: FC = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const transport = useTransport();
  const { timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS } = useSearch({
    from: "/_app/_main/observability",
  });

  const handleTimeframeChange = (timeframe: ObservabilityTimeframe) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, timeframe, runsBucket: undefined, runsStatus: undefined }),
    });
  };

  const handleRefresh = () => {
    void queryClient.invalidateQueries({
      queryKey: createQueryTimeseriesQueryKey(undefined, transport),
    });
    void queryClient.invalidateQueries({
      queryKey: createQueryAggregateQueryKey(undefined, transport),
    });
    void queryClient.invalidateQueries({ queryKey: createListRunsQueryKey(undefined, transport) });
    void queryClient.invalidateQueries({
      queryKey: createListConnectionsQueryKey(undefined, transport),
    });
    void queryClient.invalidateQueries({
      queryKey: createListPipelinesQueryKey(undefined, transport),
    });
  };

  return (
    <Topbar
      actions={
        <>
          <ObservabilityTimeframeSwitcher value={timeframe} onChange={handleTimeframeChange} />
          <Button
            icon={ArrowsClockwiseIcon}
            variant={ButtonVariant.SECONDARY}
            ariaLabel="Refresh"
            tooltip="Refresh"
            onClick={handleRefresh}
          />
        </>
      }
    >
      Observability
    </Topbar>
  );
};

export default ObservabilityToolbar;
