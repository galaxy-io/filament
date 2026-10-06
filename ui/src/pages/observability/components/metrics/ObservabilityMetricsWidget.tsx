import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { HardDrivesIcon, InfoIcon, RowsIcon } from "@phosphor-icons/react";
import { useSearch } from "@tanstack/react-router";

import StatChart, { StatChartVariant } from "@galaxy-io/dls/charts/StatChart";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import { ListRunsRequestSchema, RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric, MetricDimension, QueryAggregateRequestSchema } from "@/gen/metrics/v1/metrics_pb";

import MetricGroup from "@/components/metrics/MetricGroup";

import { OBSERVABILITY_RUN_STATUSES } from "@/pages/observability/components/runs/constants";
import { ObservabilityTimeframe } from "@/pages/observability/types";
import { createTimeframeSince } from "@/pages/observability/utils";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";
import PipelineRunStatusSwatch from "@/pages/pipelines/history/PipelineRunStatusSwatch";

import { useQueryAggregateQuery } from "@/api/queries/metrics";
import { useListRunsQuery } from "@/api/queries/runs";

import { formatBytes, formatCount } from "@/utils/format";

const OBSERVABILITY_METRICS_FEATURED_STATUSES = [
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.RUNNING,
];

const OBSERVABILITY_METRICS_OTHER_STATUSES = OBSERVABILITY_RUN_STATUSES.filter(
  (status) =>
    !OBSERVABILITY_METRICS_FEATURED_STATUSES.includes(status) && status !== RunStatus.SCHEDULED,
);

const OBSERVABILITY_METRICS_SCHEDULED_INPUT = create(ListRunsRequestSchema, {
  status: [RunStatus.SCHEDULED],
});

const ObservabilityMetricsWidget = () => {
  const { timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS } = useSearch({
    from: "/_app/_main/observability",
  });

  const { totalsInput, statusCountsInput } = useMemo(() => {
    const sinceMs = createTimeframeSince(timeframe);
    return {
      totalsInput: create(QueryAggregateRequestSchema, {
        metrics: [Metric.RUN_COUNT, Metric.RUN_RECORDS, Metric.RUN_BYTES],
        sinceMs,
      }),
      statusCountsInput: create(QueryAggregateRequestSchema, {
        metrics: [Metric.RUN_COUNT],
        sinceMs,
        groupBy: MetricDimension.STATUS,
      }),
    };
  }, [timeframe]);

  const { data: totalsData, isLoading: isTotalsLoading } = useQueryAggregateQuery({
    input: totalsInput,
  });
  const { data: statusCountsData, isLoading: isStatusCountsLoading } = useQueryAggregateQuery({
    input: statusCountsInput,
  });
  const { data: scheduledData, isLoading: isScheduledLoading } = useListRunsQuery({
    input: OBSERVABILITY_METRICS_SCHEDULED_INPUT,
  });

  const [totalRuns = 0, totalRecords = 0, totalBytes = 0] = totalsData?.rows[0]?.values ?? [];

  const countsByStatus = useMemo(
    () =>
      new Map(
        (statusCountsData?.rows ?? []).map((row) => [
          Number(row.key) as RunStatus,
          row.values[0] ?? 0,
        ]),
      ),
    [statusCountsData],
  );

  const otherStatusesCount = OBSERVABILITY_METRICS_OTHER_STATUSES.reduce(
    (sum, status) => sum + (countsByStatus.get(status) ?? 0),
    0,
  );

  return (
    <MetricGroup
      primary={
        <StatChart
          label="Total runs"
          value={formatCount(BigInt(Math.round(totalRuns)))}
          isLoading={isTotalsLoading}
        />
      }
    >
      <StatChart
        label="Total records"
        value={formatCount(BigInt(Math.round(totalRecords)))}
        trailing={<Icon component={RowsIcon} variant={IconVariant.TERTIARY} size={14} />}
        variant={StatChartVariant.TERTIARY}
        isLoading={isTotalsLoading}
        hasBorder
      />
      <StatChart
        label="Total volume"
        value={formatBytes(BigInt(Math.round(totalBytes)))}
        trailing={<Icon component={HardDrivesIcon} variant={IconVariant.TERTIARY} size={14} />}
        variant={StatChartVariant.TERTIARY}
        isLoading={isTotalsLoading}
        hasBorder
      />
      {OBSERVABILITY_METRICS_FEATURED_STATUSES.map((status) => (
        <StatChart
          key={status}
          label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[status]}
          value={formatCount(BigInt(Math.round(countsByStatus.get(status) ?? 0)))}
          trailing={<PipelineRunStatusSwatch status={status} />}
          isLoading={isStatusCountsLoading}
          hasBorder
        />
      ))}
      <StatChart
        label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED]}
        value={formatCount(BigInt(scheduledData?.runs.length ?? 0))}
        trailing={<PipelineRunStatusSwatch status={RunStatus.SCHEDULED} />}
        isLoading={isScheduledLoading}
        hasBorder
      />
      <StatChart
        label="Other"
        value={formatCount(BigInt(Math.round(otherStatusesCount)))}
        trailing={
          <Tooltip
            body={
              <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={4}>
                {OBSERVABILITY_METRICS_OTHER_STATUSES.map((status) => (
                  <Flex
                    key={status}
                    alignItems={AlignItems.CENTER}
                    justifyContent={JustifyContent.SPACE_BETWEEN}
                    gap={48}
                    fillWidth
                  >
                    <Flex alignItems={AlignItems.CENTER} gap={8}>
                      <PipelineRunStatusSwatch status={status} />
                      <Text size={TextSize.BODY_SM}>
                        {PIPELINE_RUN_STATUS_TO_LABEL_MAP[status]}
                      </Text>
                    </Flex>
                    <Text size={TextSize.BODY_SM}>
                      {formatCount(BigInt(Math.round(countsByStatus.get(status) ?? 0)))}
                    </Text>
                  </Flex>
                ))}
              </Flex>
            }
          >
            <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
          </Tooltip>
        }
        isLoading={isStatusCountsLoading}
        hasBorder
      />
    </MetricGroup>
  );
};

export default ObservabilityMetricsWidget;
