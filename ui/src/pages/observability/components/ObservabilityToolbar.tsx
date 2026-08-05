import { ArrowsClockwiseIcon } from "@phosphor-icons/react";
import { useQueryClient } from "@tanstack/react-query";
import { useNavigate, useSearch } from "@tanstack/react-router";

import Button, { ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import ObservabilityTimeframeSwitcher from "@/pages/observability/components/ObservabilityTimeframeSwitcher";
import { ObservabilityTimeframe } from "@/pages/observability/types";

import { createQueryAggregateQueryKey, createQueryTimeseriesQueryKey } from "@/api/queries/metrics";
import { createListRunsQueryKey } from "@/api/queries/runs";

const ObservabilityToolbar = () => {
  const navigate = useNavigate();
  const queryClient = useQueryClient();
  const { timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS } = useSearch({
    from: "/_main/observability",
  });

  const handleTimeframeChange = (timeframe: ObservabilityTimeframe) => {
    void navigate({ to: ".", search: (prev) => ({ ...prev, timeframe }) });
  };

  const handleRefresh = () => {
    void queryClient.invalidateQueries({ queryKey: createQueryTimeseriesQueryKey() });
    void queryClient.invalidateQueries({ queryKey: createQueryAggregateQueryKey() });
    void queryClient.invalidateQueries({ queryKey: createListRunsQueryKey() });
  };

  return (
    <BaseToolbar
      leadingActions={[
        <Text key="title" variant={TextVariant.PRIMARY} weight={TextWeight.MEDIUM}>
          Observability
        </Text>,
      ]}
      trailingActions={[
        <ObservabilityTimeframeSwitcher
          key="timeframe-switcher"
          value={timeframe}
          onChange={handleTimeframeChange}
        />,
        <Button
          key="refresh"
          icon={ArrowsClockwiseIcon}
          variant={ButtonVariant.SECONDARY}
          ariaLabel="Refresh"
          onClick={handleRefresh}
        />,
      ]}
    />
  );
};

export default ObservabilityToolbar;
