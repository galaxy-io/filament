import type { FC } from "react";

import ToggleInput, {
  ToggleInputSize,
  ToggleInputVariant,
} from "@galaxy-io/dls/inputs/ToggleInput";
import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { OBSERVABILITY_BAND_VIEW_KEY_TO_CONFIG_MAP } from "@/pages/observability/components/band/constants";
import ObservabilityBandPivotSelect from "@/pages/observability/components/band/ObservabilityBandPivotSelect";
import ObservabilityBandTile from "@/pages/observability/components/band/ObservabilityBandTile";
import ObservabilityBandTimeseriesChart from "@/pages/observability/components/band/ObservabilityBandTimeseriesChart";
import ObservabilityBandTopList from "@/pages/observability/components/band/ObservabilityBandTopList";
import type { ObservabilityBandViewConfig } from "@/pages/observability/components/band/types";
import { getObservabilityBandTopColors } from "@/pages/observability/components/band/utils";
import {
  OBSERVABILITY_BAND_VIEW_KEY_TO_DEFAULT_MAP,
  OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP,
} from "@/pages/observability/constants";
import { useObservabilityBandTotals } from "@/pages/observability/hooks/useObservabilityBandTotals";
import { useObservabilityMetricInputs } from "@/pages/observability/hooks/useObservabilityMetricInputs";
import type {
  ObservabilityBandPivotKey,
  ObservabilityBandViewKey,
} from "@/pages/observability/types";

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import type { ObservabilitySearch } from "@/module/schemas";

interface ObservabilityBandMetricTileProps {
  label: string;
  viewKey: ObservabilityBandViewKey;
  pivotKey: ObservabilityBandPivotKey;
  isStacked: boolean;
}

const ObservabilityBandMetricTile: FC<ObservabilityBandMetricTileProps> = ({
  label,
  viewKey,
  pivotKey,
  isStacked,
}) => {
  const search = useObservabilitySearch();
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const { createRankingInput } = useObservabilityMetricInputs();
  const { getTotal, isLoading } = useObservabilityBandTotals();
  const viewToConfigMap: Record<string, ObservabilityBandViewConfig> =
    OBSERVABILITY_BAND_VIEW_KEY_TO_CONFIG_MAP[viewKey];
  const view = search[viewKey];
  const pivot = search[pivotKey];
  const config = viewToConfigMap[view];

  const viewOptions = Object.entries(viewToConfigMap).map(([id, viewConfig]) => ({
    id,
    label: viewConfig.label,
  }));

  const handleViewChange = (nextView: string) => {
    void updateSearch(
      (prev) => ({
        ...prev,
        [viewKey]:
          nextView === OBSERVABILITY_BAND_VIEW_KEY_TO_DEFAULT_MAP[viewKey] ? undefined : nextView,
      }),
      { replace: true },
    );
  };

  return (
    <ObservabilityBandTile
      label={label}
      info={`${config.info} that started in the last ${OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP[search.timeframe]}.`}
      actions={
        <Flex alignItems={AlignItems.CENTER} gap={8}>
          <ToggleInput
            size={ToggleInputSize.SMALL}
            variant={ToggleInputVariant.PRIMARY}
            options={viewOptions}
            value={view}
            onChange={handleViewChange}
            ariaLabel={`${label} metric`}
          />
          <ObservabilityBandPivotSelect searchKey={pivotKey} />
        </Flex>
      }
      isLoading={isLoading}
      value={
        <Flex alignItems={AlignItems.BASELINE} gap={8} minWidth={0}>
          <Text size={TextSize.HEADING_SM} family={FontFamily.MONO}>
            {config.format(getTotal(config.metric))}
          </Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
            {config.unit}
          </Text>
        </Flex>
      }
      chart={
        <ObservabilityBandTimeseriesChart
          label={config.label}
          metric={config.metric}
          pivot={pivot}
          color={config.color}
          valueFormatter={config.axisFormat}
          isStacked={isStacked}
        />
      }
      top={
        <ObservabilityBandTopList
          title={config.topTitle}
          emptyText={config.emptyText}
          input={createRankingInput(config.metric)}
          colors={getObservabilityBandTopColors(pivot, config.color)}
          format={config.format}
        />
      }
    />
  );
};

export default ObservabilityBandMetricTile;
