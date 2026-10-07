import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";
import pluralize from "pluralize";

import MultiSelectInput, {
  MultiSelectInputSize,
  MultiSelectInputVariant,
} from "@galaxy-io/dls/inputs/MultiSelectInput";
import type { SelectOption } from "@galaxy-io/dls/inputs/SelectInput";
import ToggleInput, {
  ToggleInputSize,
  ToggleInputVariant,
  type ToggleOption,
} from "@galaxy-io/dls/inputs/ToggleInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Text from "@galaxy-io/dls/text/Text";
import Widget from "@galaxy-io/dls/widget/Widget";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

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
import ObservabilityRunsTable from "@/pages/observability/components/runs/ObservabilityRunsTable";
import { ObservabilityRunsView } from "@/pages/observability/types";
import PipelineRunStatusSwatch from "@/pages/pipelines/history/PipelineRunStatusSwatch";

import { getSelectAllChange, getSelectAllOptions, getSelectAllValue } from "@/utils/select";

const withStatusSwatch = (option: SelectOption): SelectOption => ({
  ...option,
  leading: <PipelineRunStatusSwatch status={Number(option.id) as RunStatus} />,
});

const STATUS_OPTIONS = getSelectAllOptions(
  OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION,
  OBSERVABILITY_RUN_STATUS_OPTIONS.map(withStatusSwatch),
);

const SCHEDULED_STATUS_OPTIONS = [withStatusSwatch(OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION)];

const ObservabilityRunsWidget = () => {
  const navigate = useNavigate();
  const {
    runs: view = ObservabilityRunsView.PAST,
    statuses = OBSERVABILITY_RUNS_DEFAULT_STATUSES,
  } = useSearch({
    from: "/_app/_main/observability",
  });

  const selectedStatusIds = useMemo(
    () =>
      OBSERVABILITY_RUN_STATUS_OPTIONS.map((option) => option.id).filter((id) =>
        statuses.includes(Number(id) as RunStatus),
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

  const handleStatusChange = (ids: string[]) => {
    const next = getSelectAllChange(
      OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION,
      ids,
      selectedStatusIds,
    );
    void navigate({
      to: ".",
      search: (prev) => ({
        ...prev,
        statuses: next.map((id) => Number(id) as RunStatus),
        runsBucket: undefined,
        runsStatus: undefined,
      }),
    });
  };

  const switcherItems: ToggleOption<ObservabilityRunsView>[] = Object.values(
    ObservabilityRunsView,
  ).map((runsView) => ({
    id: runsView,
    label: OBSERVABILITY_RUNS_VIEW_TO_LABEL_MAP[runsView],
  }));
  const isUpcoming = view === ObservabilityRunsView.UPCOMING;

  return (
    <Widget
      isFlush
      gap={0}
      header="Runs"
      actions={
        <>
          <ToggleInput
            size={ToggleInputSize.SMALL}
            variant={ToggleInputVariant.PRIMARY}
            options={switcherItems}
            value={view}
            onChange={handleViewChange}
          />
          <Box width={OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH}>
            <MultiSelectInput
              fillWidth
              options={isUpcoming ? SCHEDULED_STATUS_OPTIONS : STATUS_OPTIONS}
              pinnedIds={[OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION.id]}
              value={
                isUpcoming
                  ? [OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION.id]
                  : getSelectAllValue(
                      OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION,
                      selectedStatusIds,
                    )
              }
              size={MultiSelectInputSize.SMALL}
              variant={MultiSelectInputVariant.PRIMARY}
              onChange={handleStatusChange}
              placeholder="Select statuses..."
              isDisabled={isUpcoming}
              renderValue={(options) => (
                <Text>
                  {pluralize(
                    "status",
                    options.filter(
                      (option) => option.id !== OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION.id,
                    ).length,
                    true,
                  )}
                </Text>
              )}
            />
          </Box>
        </>
      }
    >
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
