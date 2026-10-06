import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";
import pluralize from "pluralize";

import MultiSelectInput, { MultiSelectInputVariant } from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import ToggleInput, {
  ToggleInputVariant,
  type ToggleOption,
} from "@galaxy-io/dls/inputs/ToggleInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
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
    from: "/_app/_main/observability",
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

  const handleStatusChange = (selected: SelectOption[]) => {
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

  const switcherItems: ToggleOption[] = Object.values(ObservabilityRunsView).map((runsView) => ({
    id: runsView,
    label: OBSERVABILITY_RUNS_VIEW_TO_LABEL_MAP[runsView],
    onClick: () => handleViewChange(runsView),
  }));

  return (
    <Widget /* @dls-migrate widget.fillWidth: Grow the card with a `FlexItem` or a `Grid` track. */
      fillWidth
      isFlush
    >
      <BaseToolbar
        leadingActions={[
          <Text key="title" variant={TextVariant.PRIMARY} weight={TextWeight.MEDIUM}>
            Runs
          </Text>,
        ]}
        trailingActions={[
          <ObservabilityRunsSelectionChips key="selection-chips" />,
          <ToggleInput
            key="view-switcher"
            variant={ToggleInputVariant.TERTIARY}
            options={switcherItems}
            value={view}
          />,
          <Box key="status-selector" width={OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH}>
            <MultiSelectInput
              fillWidth
              options={OBSERVABILITY_RUN_STATUS_OPTIONS}
              /* @dls-migrate multiselectinput.value: `value` and `onChange` now carry option ids, not option objects. */ value={
                view === ObservabilityRunsView.UPCOMING
                  ? [OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION]
                  : selectedStatusOptions
              }
              variant={MultiSelectInputVariant.TERTIARY}
              onChange={handleStatusChange}
              placeholder="Select statuses..."
              /* @dls-migrate multiselectinput.pinnedOptions: Pinned rows are now option ids: pass `pinnedIds`. */ pinnedOptions={[
                OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION,
              ]}
              isDisabled={view === ObservabilityRunsView.UPCOMING}
              /* @dls-migrate multiselectinput.renderSelectedText: Merged into `renderValue(options)`. */ renderSelectedText={(
                selectedOptions,
                placeholder,
              ) =>
                selectedOptions.length
                  ? pluralize("status", selectedOptions.length, true)
                  : placeholder
              }
            />
          </Box>,
        ]}
      />
      <Divider />
      {view === ObservabilityRunsView.PAST ? (
        <ObservabilityRunsChart />
      ) : (
        <ObservabilityRunsScheduledChart />
      )}
      <Divider />
      <ObservabilityRunsTable />
    </Widget>
  );
};

export default ObservabilityRunsWidget;
