import type { FC } from "react";

import ToggleInput, {
  ToggleInputSize,
  ToggleInputVariant,
} from "@galaxy-io/dls/inputs/ToggleInput";

import { OBSERVABILITY_RUNS_VIEW_OPTIONS } from "@/pages/observability/components/runs/constants";
import { OBSERVABILITY_DEFAULT_RUNS_VIEW } from "@/pages/observability/constants";
import type { ObservabilityRunsView } from "@/pages/observability/types";

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import type { ObservabilitySearch } from "@/module/schemas";

const ObservabilityRunsViewToggle: FC = () => {
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const { runs: view } = useObservabilitySearch();

  const handleViewChange = (nextView: ObservabilityRunsView) => {
    void updateSearch(
      (prev) => ({
        ...prev,
        runs: nextView === OBSERVABILITY_DEFAULT_RUNS_VIEW ? undefined : nextView,
      }),
      { replace: true },
    );
  };

  return (
    <ToggleInput
      size={ToggleInputSize.SMALL}
      variant={ToggleInputVariant.PRIMARY}
      options={OBSERVABILITY_RUNS_VIEW_OPTIONS}
      value={view}
      onChange={handleViewChange}
      ariaLabel="Runs view"
    />
  );
};

export default ObservabilityRunsViewToggle;
