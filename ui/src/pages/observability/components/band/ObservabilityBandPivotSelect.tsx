import type { FC } from "react";

import SelectInput, {
  SelectInputSize,
  SelectInputVariant,
} from "@galaxy-io/dls/inputs/SelectInput";
import Box from "@galaxy-io/dls/layout/Box";

import { MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import {
  OBSERVABILITY_BAND_PIVOT_OPTIONS,
  OBSERVABILITY_BAND_PIVOT_SELECT_WIDTH,
} from "@/pages/observability/components/band/constants";
import { OBSERVABILITY_BAND_PIVOT_KEY_TO_DEFAULT_MAP } from "@/pages/observability/constants";
import type { ObservabilityBandPivotKey } from "@/pages/observability/types";

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import type { ObservabilitySearch } from "@/module/schemas";

import { mapOptionIdToEnum } from "@/utils/select";

interface ObservabilityBandPivotSelectProps {
  searchKey: ObservabilityBandPivotKey;
}

const ObservabilityBandPivotSelect: FC<ObservabilityBandPivotSelectProps> = ({ searchKey }) => {
  const search = useObservabilitySearch();
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const pivot = search[searchKey];

  const handleChange = (id: string | null) => {
    const nextPivot =
      id === null ? MetricDimension.UNSPECIFIED : mapOptionIdToEnum(MetricDimension, id);
    void updateSearch(
      (prev) => ({
        ...prev,
        [searchKey]:
          nextPivot === OBSERVABILITY_BAND_PIVOT_KEY_TO_DEFAULT_MAP[searchKey]
            ? undefined
            : nextPivot,
      }),
      { replace: true },
    );
  };

  return (
    <Box width={OBSERVABILITY_BAND_PIVOT_SELECT_WIDTH}>
      <SelectInput
        fillWidth
        options={OBSERVABILITY_BAND_PIVOT_OPTIONS}
        value={pivot === MetricDimension.UNSPECIFIED ? null : String(pivot)}
        size={SelectInputSize.SMALL}
        variant={SelectInputVariant.PRIMARY}
        onChange={handleChange}
        isClearable
        placeholder="Group..."
        ariaLabel="Group by"
      />
    </Box>
  );
};

export default ObservabilityBandPivotSelect;
