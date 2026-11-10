import { type FC, useMemo } from "react";

import pluralize from "pluralize";

import MultiSelectInput, {
  MultiSelectInputSize,
  MultiSelectInputVariant,
} from "@galaxy-io/dls/inputs/MultiSelectInput";
import Box from "@galaxy-io/dls/layout/Box";
import Text from "@galaxy-io/dls/text/Text";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  OBSERVABILITY_RUNS_ALL_STATUSES_LABEL,
  OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION,
  OBSERVABILITY_RUNS_STATUS_OPTIONS,
  OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH,
  OBSERVABILITY_RUNS_UPCOMING_STATUS_OPTIONS,
} from "@/pages/observability/components/runs/constants";
import ObservabilityRunsViewToggle from "@/pages/observability/components/runs/ObservabilityRunsViewToggle";
import { ObservabilityRunsView } from "@/pages/observability/types";

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import type { ObservabilitySearch } from "@/module/schemas";

import { mapOptionIdToEnum } from "@/utils/select";

const ObservabilityRunsTableActions: FC = () => {
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const { runs: view, statuses } = useObservabilitySearch();

  const selectedStatusIds = useMemo(
    () =>
      OBSERVABILITY_RUNS_STATUS_OPTIONS.map((option) => option.id).filter((id) =>
        statuses.includes(mapOptionIdToEnum(RunStatus, id)),
      ),
    [statuses],
  );

  const handleStatusChange = (ids: string[]) => {
    void updateSearch((prev) => ({
      ...prev,
      statuses: ids.map((id) => mapOptionIdToEnum(RunStatus, id)),
    }));
  };

  const isUpcoming = view === ObservabilityRunsView.UPCOMING;

  return (
    <>
      <ObservabilityRunsViewToggle />
      <Box width={OBSERVABILITY_RUNS_STATUS_SELECT_WIDTH}>
        <MultiSelectInput
          fillWidth
          options={
            isUpcoming
              ? OBSERVABILITY_RUNS_UPCOMING_STATUS_OPTIONS
              : OBSERVABILITY_RUNS_STATUS_OPTIONS
          }
          selectAllLabel={isUpcoming ? undefined : OBSERVABILITY_RUNS_ALL_STATUSES_LABEL}
          value={isUpcoming ? [OBSERVABILITY_RUNS_SCHEDULED_STATUS_OPTION.id] : selectedStatusIds}
          size={MultiSelectInputSize.SMALL}
          variant={MultiSelectInputVariant.PRIMARY}
          onChange={handleStatusChange}
          placeholder="Select statuses..."
          ariaLabel="Run statuses"
          isDisabled={isUpcoming}
          renderValue={(options) => <Text>{pluralize("status", options.length, true)}</Text>}
        />
      </Box>
    </>
  );
};

export default ObservabilityRunsTableActions;
