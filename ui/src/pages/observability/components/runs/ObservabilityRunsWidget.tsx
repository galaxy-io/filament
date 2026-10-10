import { type FC, useMemo } from "react";

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

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

import {
  OBSERVABILITY_RUN_STATUS_OPTIONS,
  OBSERVABILITY_RUNS_ALL_STATUSES_LABEL,
  OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION,
  OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH,
  OBSERVABILITY_RUNS_VIEW_TO_LABEL_MAP,
} from "@/pages/observability/components/runs/constants";
import ObservabilityRunsChart from "@/pages/observability/components/runs/ObservabilityRunsChart";
import ObservabilityRunsScheduledChart from "@/pages/observability/components/runs/ObservabilityRunsScheduledChart";
import ObservabilityRunsTable from "@/pages/observability/components/runs/ObservabilityRunsTable";
import { ObservabilityRunsView } from "@/pages/observability/types";

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import type { ObservabilitySearch } from "@/module/schemas";

import { mapOptionIdToEnum } from "@/utils/select";

const mapOptionToSwatchOption = (option: SelectOption): SelectOption => ({
  ...option,
  leading: <PipelineRunStatusSwatch status={mapOptionIdToEnum(RunStatus, option.id)} />,
});

const STATUS_OPTIONS = OBSERVABILITY_RUN_STATUS_OPTIONS.map(mapOptionToSwatchOption);

const SCHEDULED_STATUS_OPTIONS = [
  mapOptionToSwatchOption(OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION),
];

const ObservabilityRunsWidget: FC = () => {
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const { runs: view, statuses } = useObservabilitySearch();

  const selectedStatusIds = useMemo(
    () =>
      OBSERVABILITY_RUN_STATUS_OPTIONS.map((option) => option.id).filter((id) =>
        statuses.includes(mapOptionIdToEnum(RunStatus, id)),
      ),
    [statuses],
  );

  const handleViewChange = (nextView: ObservabilityRunsView) => {
    void updateSearch((prev) => ({
      ...prev,
      runs: nextView,
      runsBucket: undefined,
      runsStatus: undefined,
    }));
  };

  const handleStatusChange = (ids: string[]) => {
    void updateSearch((prev) => ({
      ...prev,
      statuses: ids.map((id) => mapOptionIdToEnum(RunStatus, id)),
      runsBucket: undefined,
      runsStatus: undefined,
    }));
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
              selectAllLabel={isUpcoming ? undefined : OBSERVABILITY_RUNS_ALL_STATUSES_LABEL}
              value={
                isUpcoming ? [OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION.id] : selectedStatusIds
              }
              size={MultiSelectInputSize.SMALL}
              variant={MultiSelectInputVariant.PRIMARY}
              onChange={handleStatusChange}
              placeholder="Select statuses..."
              isDisabled={isUpcoming}
              renderValue={(options) => <Text>{pluralize("status", options.length, true)}</Text>}
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
