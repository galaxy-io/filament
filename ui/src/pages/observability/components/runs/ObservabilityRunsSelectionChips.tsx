import { useNavigate, useSearch } from "@tanstack/react-router";

import Chip, { ChipSize, ChipVariant } from "@galaxy-io/dls/chips/Chip";

import { ObservabilityTimeframe } from "@/pages/observability/types";
import { useBucketLabelFormatter } from "@/pages/observability/utils";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";

const ObservabilityRunsSelectionChips = () => {
  const navigate = useNavigate();
  const {
    timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS,
    runsBucket,
    runsStatus,
  } = useSearch({ from: "/_main/observability" });

  const bucketLabelFormatter = useBucketLabelFormatter(timeframe);

  const handleDismissBucket = () => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, runsBucket: undefined, runsStatus: undefined }),
    });
  };

  const handleDismissStatus = () => {
    void navigate({
      to: ".",
      search: (prev) => ({ ...prev, runsStatus: undefined }),
    });
  };

  return (
    <>
      {runsBucket !== undefined && (
        <Chip
          label={bucketLabelFormatter(String(runsBucket))}
          variant={ChipVariant.SECONDARY}
          size={ChipSize.MEDIUM}
          onDismiss={handleDismissBucket}
        />
      )}
      {runsStatus !== undefined && (
        <Chip
          label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[runsStatus]}
          variant={ChipVariant.SECONDARY}
          onDismiss={handleDismissStatus}
        />
      )}
    </>
  );
};

export default ObservabilityRunsSelectionChips;
