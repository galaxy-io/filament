import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";
import pluralize from "pluralize";

import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import {
  OBSERVABILITY_RUN_STATUS_OPTIONS,
  OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION,
  OBSERVABILITY_RUNS_DEFAULT_STATUSES,
} from "@/pages/observability/components/runs/constants";
import ObservabilityRunsChart from "@/pages/observability/components/runs/ObservabilityRunsChart";
import ObservabilityRunsTable from "@/pages/observability/components/runs/ObservabilityRunsTable";
import { ObservabilityTimeframe } from "@/pages/observability/types";

const ObservabilityRunsWidget = () => {
  const navigate = useNavigate();
  const {
    timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS,
    statuses = OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  } = useSearch({ from: "/_main/observability" });

  const statusValues = statuses as RunStatus[];

  const selectedStatusOptions = useMemo(
    () =>
      OBSERVABILITY_RUN_STATUS_OPTIONS.filter((option) =>
        statusValues.includes(option.value as RunStatus),
      ),
    [statusValues],
  );

  const handleStatusChange = (selected: SelectInputOption[]) => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, statuses: selected.map((option) => option.value as number) }),
    });
  };

  return (
    <Widget fillWidth noPadding>
      <BaseToolbar
        leadingActions={[
          <Text key="title" variant={TextVariant.PRIMARY} weight={TextWeight.MEDIUM}>
            Runs
          </Text>,
        ]}
        trailingActions={[
          <MultiSelectInput
            key="status-selector"
            options={OBSERVABILITY_RUN_STATUS_OPTIONS}
            value={selectedStatusOptions}
            onChange={handleStatusChange}
            placeholder="Select statuses..."
            width={160}
            pinnedOptions={[OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION]}
            renderSelectedText={(selectedOptions, placeholder) =>
              selectedOptions.length
                ? pluralize("status", selectedOptions.length, true)
                : placeholder
            }
          />,
        ]}
      />
      <HorizontalDivider />
      <ObservabilityRunsChart timeframe={timeframe} statuses={statusValues} />
      <HorizontalDivider />
      <ObservabilityRunsTable timeframe={timeframe} statuses={statusValues} />
    </Widget>
  );
};

export default ObservabilityRunsWidget;
