import type { FC } from "react";

import Flex, { AlignItems } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { formatBytes } from "@galaxy-io/dls/utils/format";

import { Metric } from "@/gen/metrics/v1/metrics_pb";

import { OBSERVABILITY_BAND_THROUGHPUT_COLOR } from "@/pages/observability/components/band/constants";
import ObservabilityBandTile from "@/pages/observability/components/band/ObservabilityBandTile";
import ObservabilityBandTimeseriesChart from "@/pages/observability/components/band/ObservabilityBandTimeseriesChart";
import ObservabilityBandTopList from "@/pages/observability/components/band/ObservabilityBandTopList";
import {
  formatObservabilityBandCount,
  getObservabilityBandTotal,
} from "@/pages/observability/components/band/utils";
import { OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP } from "@/pages/observability/constants";
import { useObservabilityBandInputs } from "@/pages/observability/hooks/useObservabilityBandInputs";
import { useObservabilityRefetchInterval } from "@/pages/observability/hooks/useObservabilityRefetchInterval";

import { useObservabilitySearch } from "@/module/hooks";

import { useQueryAggregateQuery } from "@/api/queries/metrics";

const ObservabilityBandThroughputTile: FC = () => {
  const refetchInterval = useObservabilityRefetchInterval();
  const { timeframe } = useObservabilitySearch();
  const { totalsInput, recordsByPipelineInput } = useObservabilityBandInputs();

  const { data, isLoading } = useQueryAggregateQuery({
    input: totalsInput,
    options: { refetchInterval },
  });

  const values = data?.rows[0]?.values ?? [];
  const records = getObservabilityBandTotal(values, Metric.RUN_RECORDS);
  const bytes = getObservabilityBandTotal(values, Metric.RUN_BYTES);

  return (
    <ObservabilityBandTile
      label="Throughput"
      info={`Records and bytes written by runs that started in the last ${OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP[timeframe]}.`}
      isLoading={isLoading}
      value={
        <Flex alignItems={AlignItems.BASELINE} gap={8} minWidth={0}>
          <Text size={TextSize.HEADING_SM} family={FontFamily.MONO}>
            {formatObservabilityBandCount(records)}
          </Text>
          <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
            records
          </Text>
          <Text size={TextSize.HEADING_SM} family={FontFamily.MONO} variant={TextVariant.SECONDARY}>
            {formatBytes(bytes)}
          </Text>
        </Flex>
      }
      chart={
        <ObservabilityBandTimeseriesChart
          label="Records"
          metric={Metric.RUN_RECORDS}
          color={OBSERVABILITY_BAND_THROUGHPUT_COLOR}
          valueFormatter={formatObservabilityBandCount}
        />
      }
      top={
        <ObservabilityBandTopList
          title="Most records"
          emptyText="No records written"
          input={recordsByPipelineInput}
          colors={[OBSERVABILITY_BAND_THROUGHPUT_COLOR]}
          format={formatObservabilityBandCount}
        />
      }
    />
  );
};

export default ObservabilityBandThroughputTile;
