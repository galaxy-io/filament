import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { HardDrivesIcon, InfoIcon, RowsIcon } from "@phosphor-icons/react";

import BigNumber, { BigNumberVariant } from "@galaxy-io/dls/charts/BigNumber";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import { formatBytes, formatNumber } from "@galaxy-io/dls/utils/format";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric, MetricDimension, QueryAggregateRequestSchema } from "@/gen/metrics/v1/metrics_pb";

import MetricGroup from "@/components/metrics/MetricGroup";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/components/runs/constants";
import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

import ObservabilityMetricsValue from "@/pages/observability/components/metrics/ObservabilityMetricsValue";
import {
  OBSERVABILITY_RUN_STATUSES,
  OBSERVABILITY_RUNS_SCHEDULED_INPUT,
} from "@/pages/observability/components/runs/constants";
import { createTimeframeSince } from "@/pages/observability/utils";

import { useObservabilitySearch } from "@/module/hooks";

import { useQueryAggregateQuery } from "@/api/queries/metrics";
import { useListRunsQuery } from "@/api/queries/runs";

import { mapOptionIdToEnum } from "@/utils/select";

const OBSERVABILITY_METRICS_FEATURED_STATUSES = [
  RunStatus.COMPLETED,
  RunStatus.FAILED,
  RunStatus.RUNNING,
];

const OBSERVABILITY_METRICS_OTHER_STATUSES = OBSERVABILITY_RUN_STATUSES.filter(
  (status) =>
    !OBSERVABILITY_METRICS_FEATURED_STATUSES.includes(status) && status !== RunStatus.SCHEDULED,
);

const ObservabilityMetricsWidget: FC = () => {
  const { timeframe } = useObservabilitySearch();

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
    input: OBSERVABILITY_RUNS_SCHEDULED_INPUT,
  });

  const [totalRuns = 0, totalRecords = 0, totalBytes = 0] = totalsData?.rows[0]?.values ?? [];

  const countsByStatus = useMemo(
    () =>
      new Map(
        (statusCountsData?.rows ?? []).map((row) => [
          mapOptionIdToEnum(RunStatus, row.key),
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
        <BigNumber
          label="Total runs"
          value={formatNumber(totalRuns, { precision: 0 })}
          isLoading={isTotalsLoading}
        />
      }
    >
      <BigNumber
        label="Total records"
        value={
          <ObservabilityMetricsValue
            value={formatNumber(totalRecords, { precision: 0 })}
            mark={<Icon component={RowsIcon} variant={IconVariant.TERTIARY} size={14} />}
          />
        }
        variant={BigNumberVariant.TERTIARY}
        isLoading={isTotalsLoading}
        hasBorder
      />
      <BigNumber
        label="Total volume"
        value={
          <ObservabilityMetricsValue
            value={formatBytes(totalBytes)}
            mark={<Icon component={HardDrivesIcon} variant={IconVariant.TERTIARY} size={14} />}
          />
        }
        variant={BigNumberVariant.TERTIARY}
        isLoading={isTotalsLoading}
        hasBorder
      />
      {OBSERVABILITY_METRICS_FEATURED_STATUSES.map((status) => (
        <BigNumber
          key={status}
          label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[status]}
          value={
            <ObservabilityMetricsValue
              value={formatNumber(countsByStatus.get(status) ?? 0, { precision: 0 })}
              mark={<PipelineRunStatusSwatch status={status} />}
            />
          }
          isLoading={isStatusCountsLoading}
          hasBorder
        />
      ))}
      <BigNumber
        label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED]}
        value={
          <ObservabilityMetricsValue
            value={formatNumber(scheduledData?.runs.length ?? 0)}
            mark={<PipelineRunStatusSwatch status={RunStatus.SCHEDULED} />}
          />
        }
        isLoading={isScheduledLoading}
        hasBorder
      />
      <BigNumber
        label="Other"
        value={
          <ObservabilityMetricsValue
            value={formatNumber(otherStatusesCount, { precision: 0 })}
            mark={
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
                          {formatNumber(countsByStatus.get(status) ?? 0, { precision: 0 })}
                        </Text>
                      </Flex>
                    ))}
                  </Flex>
                }
              >
                <Icon component={InfoIcon} variant={IconVariant.TERTIARY} size={14} />
              </Tooltip>
            }
          />
        }
        isLoading={isStatusCountsLoading}
        hasBorder
      />
    </MetricGroup>
  );
};

export default ObservabilityMetricsWidget;
