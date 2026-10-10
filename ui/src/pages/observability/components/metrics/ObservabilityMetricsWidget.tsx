import { type FC, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { HardDrivesIcon, InfoIcon, RowsIcon } from "@phosphor-icons/react";

import BigNumber from "@galaxy-io/dls/charts/BigNumber";
import BigNumberGroup, { BigNumberGroupVariant } from "@galaxy-io/dls/charts/BigNumberGroup";
import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import { formatBytes, formatNumber } from "@galaxy-io/dls/utils/format";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric, MetricDimension, QueryAggregateRequestSchema } from "@/gen/metrics/v1/metrics_pb";

import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/components/runs/constants";
import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

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
    <BigNumberGroup
      ariaLabel="Run metrics"
      variant={BigNumberGroupVariant.PRIMARY}
      hasBorder
      hasFadeEdges
      fillWidth
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
        value={formatNumber(totalRecords, { precision: 0 })}
        suffix={<Icon component={RowsIcon} variant={IconVariant.TERTIARY} size={14} />}
        isLoading={isTotalsLoading}
      />
      <BigNumber
        label="Total volume"
        value={formatBytes(totalBytes)}
        suffix={<Icon component={HardDrivesIcon} variant={IconVariant.TERTIARY} size={14} />}
        isLoading={isTotalsLoading}
      />
      {OBSERVABILITY_METRICS_FEATURED_STATUSES.map((status) => (
        <BigNumber
          key={status}
          label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[status]}
          value={formatNumber(countsByStatus.get(status) ?? 0, { precision: 0 })}
          suffix={<PipelineRunStatusSwatch status={status} />}
          isLoading={isStatusCountsLoading}
        />
      ))}
      <BigNumber
        label={PIPELINE_RUN_STATUS_TO_LABEL_MAP[RunStatus.SCHEDULED]}
        value={formatNumber(scheduledData?.runs.length ?? 0, { precision: 0 })}
        suffix={<PipelineRunStatusSwatch status={RunStatus.SCHEDULED} />}
        isLoading={isScheduledLoading}
      />
      <BigNumber
        label="Other"
        value={formatNumber(otherStatusesCount, { precision: 0 })}
        suffix={
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
        isLoading={isStatusCountsLoading}
      />
    </BigNumberGroup>
  );
};

export default ObservabilityMetricsWidget;
