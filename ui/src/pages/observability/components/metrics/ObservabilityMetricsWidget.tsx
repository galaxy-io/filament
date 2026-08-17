import { useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { HardDrivesIcon, InfoIcon, RowsIcon } from "@phosphor-icons/react";
import { useSearch } from "@tanstack/react-router";

import Beacon from "@galaxy-io/dls/beacons/Beacon";
import { StatChartVariant } from "@galaxy-io/dls/charts/StatChart";
import FlexWrapper, {
  AlignItems,
  FlexDirection,
  FlexGap,
  JustifyContent,
} from "@galaxy-io/dls/containers/FlexWrapper";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Text, { TextSize, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import { ListRunsRequestSchema, RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric, MetricDimension, QueryAggregateRequestSchema } from "@/gen/metrics/v1/metrics_pb";

import MetricCard from "@/components/metrics/MetricCard";
import MetricGroup from "@/components/metrics/MetricGroup";

import {
  OBSERVABILITY_RUN_STATUSES,
  OBSERVABILITY_RUNS_TABLE_LIMIT,
} from "@/pages/observability/components/runs/constants";
import { ObservabilityTimeframe } from "@/pages/observability/types";
import { createTimeframeSince } from "@/pages/observability/utils";
import {
  PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/pages/pipelines/history/constants";

import { useQueryAggregateQuery } from "@/api/queries/metrics";
import { useListRunsQuery } from "@/api/queries/runs";

import { formatBytes, formatCount } from "@/utils/format";

const OBSERVABILITY_METRICS_FEATURED_STATUSES = [
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.RUNNING,
];

// Scheduled is excluded: it has its own card fed by ListRuns, since scheduled
// runs never started and so can't appear in the windowed aggregate.
const OBSERVABILITY_METRICS_OTHER_STATUSES = OBSERVABILITY_RUN_STATUSES.filter(
  (status) =>
    !OBSERVABILITY_METRICS_FEATURED_STATUSES.includes(status) && status !== RunStatus.SCHEDULED,
);

const OBSERVABILITY_METRICS_SCHEDULED_INPUT = create(ListRunsRequestSchema, {
  status: [RunStatus.SCHEDULED],
  pagination: create(PaginationRequestSchema, { pageSize: OBSERVABILITY_RUNS_TABLE_LIMIT }),
});

const ObservabilityMetricsWidget = () => {
  const { timeframe = ObservabilityTimeframe.TWENTY_FOUR_HOURS } = useSearch({
    from: "/_main/observability",
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

  const totalValue = (value: string) =>
    isTotalsLoading ? <TextShimmer width={48} height={18} /> : value;

  return (
    <MetricGroup
      primary={
        <MetricCard
          label="Total runs"
          value={totalValue(formatCount(BigInt(Math.round(totalRuns))))}
          noBorder
        />
      }
    >
      <MetricCard
        label="Total records"
        value={totalValue(formatCount(BigInt(Math.round(totalRecords))))}
        icon={RowsIcon}
        variant={StatChartVariant.TERTIARY}
      />
      <MetricCard
        label="Total volume"
        value={totalValue(formatBytes(BigInt(Math.round(totalBytes))))}
        icon={HardDrivesIcon}
        variant={StatChartVariant.TERTIARY}
      />
      {OBSERVABILITY_METRICS_FEATURED_STATUSES.map((status) => (
        <MetricCard
          key={status}
          label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[status]}
          value={
            <FlexWrapper alignItems={AlignItems.CENTER} gap={6}>
              {isStatusCountsLoading ? (
                <TextShimmer width={32} height={18} />
              ) : (
                <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
                  {formatCount(BigInt(Math.round(countsByStatus.get(status) ?? 0)))}
                </Text>
              )}
              <Beacon variant={PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP[status]} />
            </FlexWrapper>
          }
        />
      ))}
      <MetricCard
        label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED]}
        value={
          <FlexWrapper alignItems={AlignItems.CENTER} gap={6}>
            {isScheduledLoading ? (
              <TextShimmer width={32} height={18} />
            ) : (
              <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
                {formatCount(BigInt(scheduledData?.runs.length ?? 0))}
              </Text>
            )}
            <Beacon variant={PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP[RunStatus.SCHEDULED]} />
          </FlexWrapper>
        }
      />
      <MetricCard
        label="Other"
        value={
          <FlexWrapper alignItems={AlignItems.CENTER} gap={6}>
            {isStatusCountsLoading ? (
              <TextShimmer width={32} height={18} />
            ) : (
              <Text size={TextSize.BODY_LG} weight={TextWeight.MEDIUM}>
                {formatCount(BigInt(Math.round(otherStatusesCount)))}
              </Text>
            )}
            <Tooltip
              body={
                <FlexWrapper direction={FlexDirection.COLUMN} gap={6}>
                  {OBSERVABILITY_METRICS_OTHER_STATUSES.map((status) => (
                    <FlexWrapper
                      key={status}
                      alignItems={AlignItems.CENTER}
                      justifyContent={JustifyContent.SPACE_BETWEEN}
                      gap={48}
                      fillWidth
                    >
                      <FlexWrapper alignItems={AlignItems.CENTER} gap={FlexGap.SMALL}>
                        <Beacon variant={PIPELINE_RUN_STATUS_TO_BEACON_VARIANT_MAP[status]} />
                        <Text size={TextSize.BODY_SM}>
                          {PIPELINE_RUN_STATUS_TO_LABEL_MAP[status]}
                        </Text>
                      </FlexWrapper>
                      <Text size={TextSize.BODY_SM}>
                        {formatCount(BigInt(Math.round(countsByStatus.get(status) ?? 0)))}
                      </Text>
                    </FlexWrapper>
                  ))}
                </FlexWrapper>
              }
            >
              <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
            </Tooltip>
          </FlexWrapper>
        }
      />
    </MetricGroup>
  );
};

export default ObservabilityMetricsWidget;
