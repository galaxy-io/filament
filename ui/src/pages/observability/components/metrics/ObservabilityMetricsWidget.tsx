import { HardDrivesIcon, RowsIcon } from "@phosphor-icons/react";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import { StatChartVariant } from "@galaxy-io/dls/charts/StatChart";
import FlexWrapper, { AlignItems } from "@galaxy-io/dls/containers/FlexWrapper";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";

import MetricCard from "@/components/metrics/MetricCard";
import MetricGroup from "@/components/metrics/MetricGroup";

import { OBSERVABILITY_TIMEFRAME_TO_METRICS_MAP } from "@/pages/observability/components/metrics/constants";
import { OBSERVABILITY_RUN_STATUSES } from "@/pages/observability/components/runs/constants";
import { useObservabilityTimeframe } from "@/pages/observability/providers/ObservabilityTimeframeProvider";
import {
  PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/pages/pipelines/history/constants";

import { formatBytes, formatCount } from "@/utils/format";

const ObservabilityMetricsWidget = () => {
  const { timeframe } = useObservabilityTimeframe();

  const metrics = OBSERVABILITY_TIMEFRAME_TO_METRICS_MAP[timeframe];
  const totalRuns = OBSERVABILITY_RUN_STATUSES.reduce(
    (sum, status) => sum + (metrics.statusCounts[status] ?? 0),
    0,
  );

  return (
    <MetricGroup
      primary={<MetricCard label="Total runs" value={formatCount(BigInt(totalRuns))} noBorder />}
    >
      <MetricCard
        label="Total records"
        value={formatCount(metrics.totalRecords)}
        icon={RowsIcon}
        variant={StatChartVariant.TERTIARY}
      />
      <MetricCard
        label="Total volume"
        value={formatBytes(metrics.totalBytes)}
        icon={HardDrivesIcon}
        variant={StatChartVariant.TERTIARY}
      />
      {OBSERVABILITY_RUN_STATUSES.map((status) => (
        <MetricCard
          key={status}
          label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[status]}
          value={
            <FlexWrapper alignItems={AlignItems.CENTER} gap={6}>
              <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
                {formatCount(BigInt(metrics.statusCounts[status] ?? 0))}
              </Text>
              <Beacon variant={PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP[status]} />
            </FlexWrapper>
          }
        />
      ))}
    </MetricGroup>
  );
};

export default ObservabilityMetricsWidget;
