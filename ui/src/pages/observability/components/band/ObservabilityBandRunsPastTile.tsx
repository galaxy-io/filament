import { type FC, useMemo } from "react";

import { styled } from "@linaria/react";
import { InfoIcon } from "@phosphor-icons/react";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Flex, { AlignItems, FlexDirection, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import { SquareSize } from "@galaxy-io/dls/shapes/Square";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric, MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import {
  PIPELINE_RUN_EXECUTED_STATUSES,
  PIPELINE_RUN_STATUS_TO_HUE_MAP,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/components/runs/constants";
import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

import {
  OBSERVABILITY_BAND_COUNTS_HEIGHT,
  OBSERVABILITY_BAND_FEATURED_STATUSES,
  OBSERVABILITY_BAND_INFO_ICON_SIZE,
  OBSERVABILITY_BAND_OTHER_STATUSES,
} from "@/pages/observability/components/band/constants";
import ObservabilityBandRunsChart from "@/pages/observability/components/band/ObservabilityBandRunsChart";
import ObservabilityBandTile from "@/pages/observability/components/band/ObservabilityBandTile";
import ObservabilityBandTopList from "@/pages/observability/components/band/ObservabilityBandTopList";
import {
  formatObservabilityBandCount,
  mapObservabilityBandRunsGroups,
} from "@/pages/observability/components/band/utils";
import ObservabilityRunsViewToggle from "@/pages/observability/components/runs/ObservabilityRunsViewToggle";
import { OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP } from "@/pages/observability/constants";
import { useObservabilityAggregateQuery } from "@/pages/observability/hooks/useObservabilityAggregateQuery";
import { useObservabilityMetricInputs } from "@/pages/observability/hooks/useObservabilityMetricInputs";
import { useObservabilityTimeseriesQuery } from "@/pages/observability/hooks/useObservabilityTimeseriesQuery";

import { useObservabilitySearch } from "@/module/hooks";

import { mapOptionIdToEnum } from "@/utils/select";

const ObservabilityBandRunsCounts = styled.div`
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  column-gap: ${t.space[12]};
  min-width: 0;
  height: ${OBSERVABILITY_BAND_COUNTS_HEIGHT}px;
  overflow: hidden;

  & > * {
    height: ${OBSERVABILITY_BAND_COUNTS_HEIGHT}px;
  }
`;

const ObservabilityBandRunsPastTile: FC = () => {
  const { timeframe } = useObservabilitySearch();
  const { createAggregateInput, createSeriesInput } = useObservabilityMetricInputs();
  const { data, isLoading } = useObservabilityAggregateQuery(
    createAggregateInput(Metric.RUN_COUNT, MetricDimension.STATUS),
  );
  const { data: seriesData, isFetching } = useObservabilityTimeseriesQuery(
    createSeriesInput(Metric.RUN_COUNT, MetricDimension.STATUS, PIPELINE_RUN_EXECUTED_STATUSES),
  );
  const groups = useMemo(
    () => mapObservabilityBandRunsGroups(seriesData?.series ?? []),
    [seriesData],
  );

  const counts = useMemo(
    () =>
      new Map(
        (data?.rows ?? []).map((row) => [
          mapOptionIdToEnum(RunStatus, row.key),
          row.values[0] ?? 0,
        ]),
      ),
    [data],
  );

  const total = [...counts.values()].reduce((sum, count) => sum + count, 0);
  const otherCount = OBSERVABILITY_BAND_OTHER_STATUSES.reduce(
    (sum, status) => sum + (counts.get(status) ?? 0),
    0,
  );
  const featured = OBSERVABILITY_BAND_FEATURED_STATUSES.map((status) => ({
    status,
    count: counts.get(status) ?? 0,
  })).filter(({ count }) => count > 0);

  return (
    <ObservabilityBandTile
      label="Runs"
      info={`Runs that started in the last ${OBSERVABILITY_TIMEFRAME_TO_LABEL_MAP[timeframe]}, by status.`}
      actions={<ObservabilityRunsViewToggle />}
      isLoading={isLoading}
      value={
        <Flex alignItems={AlignItems.CENTER} gap={12} minWidth={0}>
          <FlexItem shrink={0}>
            <Text size={TextSize.HEADING_SM} family={FontFamily.MONO}>
              {formatObservabilityBandCount(total)}
            </Text>
          </FlexItem>
          <ObservabilityBandRunsCounts>
            {featured.map(({ status, count }) => (
              <Flex key={status} alignItems={AlignItems.CENTER} gap={4}>
                <PipelineRunStatusSwatch status={status} size={SquareSize.SMALL} />
                <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY} lineClamp={1}>
                  {`${formatObservabilityBandCount(count)} ${PIPELINE_RUN_STATUS_TO_LABEL_MAP[status].toLowerCase()}`}
                </Text>
              </Flex>
            ))}
            {otherCount > 0 && (
              <Flex alignItems={AlignItems.CENTER} gap={4}>
                <Text size={TextSize.BODY_SM} variant={TextVariant.SECONDARY} lineClamp={1}>
                  {`${formatObservabilityBandCount(otherCount)} other`}
                </Text>
                <Icon
                  component={InfoIcon}
                  variant={IconVariant.TERTIARY}
                  size={OBSERVABILITY_BAND_INFO_ICON_SIZE}
                  ariaLabel="Other statuses"
                  tooltip={
                    <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={4}>
                      {OBSERVABILITY_BAND_OTHER_STATUSES.map((status) => (
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
                            {formatObservabilityBandCount(counts.get(status) ?? 0)}
                          </Text>
                        </Flex>
                      ))}
                    </Flex>
                  }
                />
              </Flex>
            )}
          </ObservabilityBandRunsCounts>
        </Flex>
      }
      chart={
        <ObservabilityBandRunsChart
          groups={groups}
          isLoading={isFetching}
          ariaLabel="Runs by status"
        />
      }
      top={
        <ObservabilityBandTopList
          title="Most failures"
          emptyText="No failed runs"
          input={createAggregateInput(Metric.RUN_COUNT, MetricDimension.PIPELINE_ID, [
            RunStatus.FAILED,
          ])}
          colors={[PIPELINE_RUN_STATUS_TO_HUE_MAP[RunStatus.FAILED]]}
          format={formatObservabilityBandCount}
        />
      }
    />
  );
};

export default ObservabilityBandRunsPastTile;
