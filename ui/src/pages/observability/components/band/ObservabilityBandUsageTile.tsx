import type { FC } from "react";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";

import { Metric } from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_BAND_PIPELINE_COLORS } from "@/pages/observability/components/band/constants";
import ObservabilityBandPipelinesChart from "@/pages/observability/components/band/ObservabilityBandPipelinesChart";
import ObservabilityBandTile from "@/pages/observability/components/band/ObservabilityBandTile";
import ObservabilityBandTopList from "@/pages/observability/components/band/ObservabilityBandTopList";
import {
  formatObservabilityBandCpu,
  formatObservabilityBandCpuAxis,
  formatObservabilityBandMemory,
  getObservabilityBandTotal,
} from "@/pages/observability/components/band/utils";
import { OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP } from "@/pages/observability/constants";
import { useObservabilityBandInputs } from "@/pages/observability/hooks/useObservabilityBandInputs";
import { useObservabilityRefetchInterval } from "@/pages/observability/hooks/useObservabilityRefetchInterval";

import { useObservabilitySearch } from "@/module/hooks";

import { useQueryAggregateQuery } from "@/api/queries/metrics";

const ObservabilityBandUsageTile: FC = () => {
  const refetchInterval = useObservabilityRefetchInterval();
  const { timeframe } = useObservabilitySearch();
  const { totalsInput, cpuByPipelineInput } = useObservabilityBandInputs();

  const { data, isLoading } = useQueryAggregateQuery({
    input: totalsInput,
    options: { refetchInterval },
  });

  const values = data?.rows[0]?.values ?? [];
  const cpu = getObservabilityBandTotal(values, Metric.RUN_CPU_USAGE);
  const memory = getObservabilityBandTotal(values, Metric.RUN_MEMORY_USAGE);

  return (
    <ObservabilityBandTile
      label="Usage"
      info={`Average CPU time and peak memory of one run, across runs that started in the last ${OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP[timeframe]}.`}
      isLoading={isLoading}
      value={
        <Flex alignItems={AlignItems.BASELINE} gap={8} minWidth={0}>
          <Text size={TextSize.HEADING_SM} family={FontFamily.MONO}>
            {formatObservabilityBandCpu(cpu)}
          </Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            CPU
          </Text>
          <Text size={TextSize.HEADING_SM} family={FontFamily.MONO} variant={TextVariant.SECONDARY}>
            {formatObservabilityBandMemory(memory)}
          </Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
            peak memory
          </Text>
        </Flex>
      }
      chart={
        <ObservabilityBandPipelinesChart
          label="CPU"
          metric={Metric.RUN_CPU_USAGE}
          rankingInput={cpuByPipelineInput}
          valueFormatter={formatObservabilityBandCpuAxis}
        />
      }
      top={
        <ObservabilityBandTopList
          title="Heaviest runs"
          emptyText="No usage reported"
          input={cpuByPipelineInput}
          colors={OBSERVABILITY_BAND_PIPELINE_COLORS}
          format={formatObservabilityBandCpu}
        />
      }
    />
  );
};

export default ObservabilityBandUsageTile;
