import { useMemo } from "react";

import { useNavigate, useSearch } from "@tanstack/react-router";
import pluralize from "pluralize";

import MultiSelectInput, { MultiSelectInputVariant } from "@galaxy-io/dls/inputs/MultiSelectInput";
import ToggleInput, {
  ToggleInputVariant,
  type ToggleOption,
} from "@galaxy-io/dls/inputs/ToggleInput";
import Box from "@galaxy-io/dls/layout/Box";
import Divider from "@galaxy-io/dls/layout/Divider";
import Flex, { FlexDirection } from "@galaxy-io/dls/layout/Flex";
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
import ObservabilityRunsTable from "@/pages/observability/components/runs/ObservabilityRunsTable";
import { ObservabilityRunsView } from "@/pages/observability/types";

import { getSelectAllChange, getSelectAllOptions, getSelectAllValue } from "@/utils/select";

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
    <Widget isFlush>
      <Flex direction={FlexDirection.COLUMN}>
        <BaseToolbar
          leadingActions={[
            <Text key="title" variant={TextVariant.PRIMARY} weight={TextWeight.MEDIUM}>
              Runs
            </Text>,
          ]}
          trailingActions={[
            <ToggleInput
              key="view-switcher"
              variant={ToggleInputVariant.PRIMARY}
              options={switcherItems}
              value={view}
              onChange={handleViewChange}
            />,
            <Box key="status-selector" width={OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH}>
              <MultiSelectInput
                fillWidth
                options={
                  isUpcoming
                    ? [OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION]
                    : getSelectAllOptions(
                        OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION,
                        OBSERVABILITY_RUN_STATUS_OPTIONS,
                      )
                }
                pinnedIds={[OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION.id]}
                value={
                  isUpcoming
                    ? [OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION.id]
                    : getSelectAllValue(
                        OBSERVABILITY_RUNS_ALL_STATUSES_PINNED_OPTION,
                        selectedStatusIds,
                      )
                }
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
      </Flex>
    </Widget>
  );
};

export default ObservabilityRunsWidget;
