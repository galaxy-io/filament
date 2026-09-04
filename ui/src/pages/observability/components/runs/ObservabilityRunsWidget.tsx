import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";
import pluralize from "pluralize";

import HorizontalDivider from "@galaxy-io/dls/dividers/HorizontalDivider";
import { InputVariant } from "@galaxy-io/dls/inputs/Input";
import MultiSelectInput from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectInputOption } from "@galaxy-io/dls/inputs/SelectInput";
import SwitcherInput, { type SwitcherInputItem } from "@galaxy-io/dls/inputs/SwitcherInput";
import Text, { TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import BaseToolbar from "@/layouts/components/BaseToolbar";

import {
  OBSERVABILITY_RUN_STATUS_OPTIONS,
  OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION,
  OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION,
  OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH,
  OBSERVABILITY_RUNS_VIEW_TO_LABEL_MAP,
} from "@/pages/observability/components/runs/constants";
import ObservabilityRunsChart from "@/pages/observability/components/runs/ObservabilityRunsChart";
import ObservabilityRunsScheduledChart from "@/pages/observability/components/runs/ObservabilityRunsScheduledChart";
import ObservabilityRunsSelectionChips from "@/pages/observability/components/runs/ObservabilityRunsSelectionChips";
import ObservabilityRunsTable from "@/pages/observability/components/runs/ObservabilityRunsTable";
import { ObservabilityRunsView } from "@/pages/observability/types";

const ObservabilityRunsWidget = () => {
  const navigate = useNavigate();
  const {
    runs: view = ObservabilityRunsView.PAST,
    statuses = OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  } = useSearch({
    from: "/_main/observability",
  });

  const selectedStatusOptions = useMemo(
    () =>
      OBSERVABILITY_RUN_STATUS_OPTIONS.filter((option) =>
        statuses.includes(option.value as RunStatus),
      ),
    [statuses],
  );

  const handleViewChange = (nextView: ObservabilityRunsView) => {
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        runs: nextView,
        runsBucket: undefined,
        runsStatus: undefined,
      }),
    });
  };

  const handleStatusChange = (selected: SelectInputOption[]) => {
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        statuses: selected.map((option) => option.value as RunStatus),
        runsBucket: undefined,
        runsStatus: undefined,
      }),
    });
  };

  const switcherItems: SwitcherInputItem[] = Object.values(ObservabilityRunsView).map(
    (runsView) => ({
      id: runsView,
      label: OBSERVABILITY_RUNS_VIEW_TO_LABEL_MAP[runsView],
      onClick: () => handleViewChange(runsView),
    }),
  );

  return (
    <Widget fillWidth noPadding>
      <BaseToolbar
        leadingActions={[
          <Text key="title" variant={TextVariant.PRIMARY} weight={TextWeight.MEDIUM}>
            Runs
          </Text>,
        ]}
        trailingActions={[
          <ObservabilityRunsSelectionChips key="selection-chips" />,
          <SwitcherInput
            key="view-switcher"
            variant={InputVariant.TERTIARY}
            items={switcherItems}
            selectedId={view}
          />,
          <MultiSelectInput
            key="status-selector"
            options={OBSERVABILITY_RUN_STATUS_OPTIONS}
            value={
              view === ObservabilityRunsView.UPCOMING
                ? [OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION]
                : selectedStatusOptions
            }
            variant={InputVariant.TERTIARY}
            onChange={handleStatusChange}
            placeholder="Select statuses..."
            width={OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH}
            pinnedOptions={[OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION]}
            isDisabled={view === ObservabilityRunsView.UPCOMING}
            renderSelectedText={(selectedOptions, placeholder) =>
              selectedOptions.length
                ? pluralize("status", selectedOptions.length, true)
                : placeholder
            }
          />,
        ]}
      />
      <HorizontalDivider />
      {view === ObservabilityRunsView.PAST ? (
        <ObservabilityRunsChart />
      ) : (
        <ObservabilityRunsScheduledChart />
      )}
      <HorizontalDivider />
      <ObservabilityRunsTable />
    </Widget>
  );
};

export default ObservabilityRunsWidget;
