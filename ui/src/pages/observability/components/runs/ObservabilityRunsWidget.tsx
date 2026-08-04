import { useMemo, useState } from "react";

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
  OBSERVABILITY_RUNS_DEFAULT_STATUS_OPTIONS,
} from "@/pages/observability/components/runs/constants";
import ObservabilityRunsChart from "@/pages/observability/components/runs/ObservabilityRunsChart";
import ObservabilityRunsTable from "@/pages/observability/components/runs/ObservabilityRunsTable";
import { useObservabilityTimeframe } from "@/pages/observability/providers/ObservabilityTimeframeProvider";

interface ObservabilityRunsWidgetState {
  selectedStatuses: SelectInputOption[];
}

const DEFAULT_STATE: ObservabilityRunsWidgetState = {
  selectedStatuses: OBSERVABILITY_RUNS_DEFAULT_STATUS_OPTIONS,
};

const ObservabilityRunsWidget = () => {
  const { timeframe } = useObservabilityTimeframe();
  const [state, setState] = useState<ObservabilityRunsWidgetState>(DEFAULT_STATE);

  const selectedStatusValues = useMemo(
    () => state.selectedStatuses.map((option) => Number(option.id) as RunStatus),
    [state.selectedStatuses],
  );

  const handleStatusChange = (selectedStatuses: SelectInputOption[]) => {
    setState((prev) => ({ ...prev, selectedStatuses }));
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
            value={state.selectedStatuses}
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
      <ObservabilityRunsChart timeframe={timeframe} statuses={selectedStatusValues} />
      <HorizontalDivider />
      <ObservabilityRunsTable statuses={selectedStatusValues} />
    </Widget>
  );
};

export default ObservabilityRunsWidget;
