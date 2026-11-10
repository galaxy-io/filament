import { type FC, useMemo } from "react";

import { styled } from "@linaria/react";
import {
  ArrowRightIcon,
  CaretLeftIcon,
  CaretRightIcon,
  FlowArrowIcon,
} from "@phosphor-icons/react";

import Button, { ButtonSize, ButtonVariant } from "@galaxy-io/dls/buttons/Button";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Widget from "@galaxy-io/dls/widget/Widget";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";
import { Metric, MetricDimension } from "@/gen/metrics/v1/metrics_pb";

import RouterLink from "@/components/RouterLink";

import ObservabilityWidgetEmpty from "@/pages/observability/components/ObservabilityWidgetEmpty";
import {
  OBSERVABILITY_PIPELINES_COLUMNS_NAME_AND_TRACK,
  OBSERVABILITY_PIPELINES_COLUMNS_NO_LAST_RUN,
  OBSERVABILITY_PIPELINES_COLUMNS_NO_RECORDS,
  OBSERVABILITY_PIPELINES_CONTAINER_NAME,
  OBSERVABILITY_PIPELINES_FOOTER_HEIGHT,
  OBSERVABILITY_PIPELINES_HIDE_FLOW_MAX_WIDTH,
  OBSERVABILITY_PIPELINES_HIDE_LAST_RUN_MAX_WIDTH,
  OBSERVABILITY_PIPELINES_HIDE_RECORDS_MAX_WIDTH,
  OBSERVABILITY_PIPELINES_LIMIT,
  OBSERVABILITY_PIPELINES_PAGE_MIN_HEIGHT,
} from "@/pages/observability/components/pipelines/constants";
import ObservabilityPipelinesHeader from "@/pages/observability/components/pipelines/ObservabilityPipelinesHeader";
import ObservabilityPipelinesRow from "@/pages/observability/components/pipelines/ObservabilityPipelinesRow";
import { ObservabilityPipelinesColumn } from "@/pages/observability/components/pipelines/types";
import {
  createObservabilityPipelinesCells,
  getObservabilityPipelinesTrackWidth,
  mapObservabilityPipelinesSeries,
} from "@/pages/observability/components/pipelines/utils";
import { OBSERVABILITY_DEFAULT_PIPELINES_PAGE } from "@/pages/observability/constants";
import { useObservabilityMetricInputs } from "@/pages/observability/hooks/useObservabilityMetricInputs";
import { useObservabilityPipelines } from "@/pages/observability/hooks/useObservabilityPipelines";
import { useObservabilityTimeseriesQuery } from "@/pages/observability/hooks/useObservabilityTimeseriesQuery";
import { getObservabilityBucketCount } from "@/pages/observability/utils";

import { useFilamentSearchUpdate, useObservabilitySearch } from "@/module/hooks";
import { createFilamentHref, FilamentPath } from "@/module/paths";
import type { ObservabilitySearch } from "@/module/schemas";

const ObservabilityPipelinesFrame = styled.div<{ $trackWidth: number }>`
  --observability-pipelines-track: ${({ $trackWidth }) => `${$trackWidth}px`};
  container: ${OBSERVABILITY_PIPELINES_CONTAINER_NAME} / inline-size;
  width: 100%;
  background-color: ${t.color.background.primary};

  @container ${OBSERVABILITY_PIPELINES_CONTAINER_NAME} (max-width: ${OBSERVABILITY_PIPELINES_HIDE_RECORDS_MAX_WIDTH}px) {
    [data-pipelines-grid] {
      grid-template-columns: ${OBSERVABILITY_PIPELINES_COLUMNS_NO_RECORDS};
    }

    [data-pipelines-column="${ObservabilityPipelinesColumn.RECORDS}"] {
      display: none;
    }
  }

  @container ${OBSERVABILITY_PIPELINES_CONTAINER_NAME} (max-width: ${OBSERVABILITY_PIPELINES_HIDE_LAST_RUN_MAX_WIDTH}px) {
    [data-pipelines-grid] {
      grid-template-columns: ${OBSERVABILITY_PIPELINES_COLUMNS_NO_LAST_RUN};
    }

    [data-pipelines-column="${ObservabilityPipelinesColumn.LAST_RUN}"] {
      display: none;
    }
  }

  @container ${OBSERVABILITY_PIPELINES_CONTAINER_NAME} (max-width: ${OBSERVABILITY_PIPELINES_HIDE_FLOW_MAX_WIDTH}px) {
    [data-pipelines-grid] {
      grid-template-columns: ${OBSERVABILITY_PIPELINES_COLUMNS_NAME_AND_TRACK};
    }

    [data-pipelines-column="${ObservabilityPipelinesColumn.FLOW}"] {
      display: none;
    }
  }
`;

const ObservabilityPipelinesWidget: FC = () => {
  const updateSearch = useFilamentSearchUpdate<ObservabilitySearch>();
  const { timeframe, pipelinesPage } = useObservabilitySearch();
  const { bucketStartMs, createSeriesInput } = useObservabilityMetricInputs();
  const { entries } = useObservabilityPipelines();
  const pageCount = Math.max(1, Math.ceil(entries.length / OBSERVABILITY_PIPELINES_LIMIT));
  const page = Math.min(pipelinesPage, pageCount);
  const pageStart = (page - 1) * OBSERVABILITY_PIPELINES_LIMIT;
  const hasPages = pageCount > 1;

  const { data: completedData } = useObservabilityTimeseriesQuery(
    createSeriesInput(Metric.RUN_COUNT, MetricDimension.PIPELINE_ID, [RunStatus.COMPLETED]),
  );
  const { data: failedData } = useObservabilityTimeseriesQuery(
    createSeriesInput(Metric.RUN_COUNT, MetricDimension.PIPELINE_ID, [RunStatus.FAILED]),
  );

  const rows = useMemo(() => {
    const completedCounts = mapObservabilityPipelinesSeries(completedData?.series ?? []);
    const failedCounts = mapObservabilityPipelinesSeries(failedData?.series ?? []);
    return entries
      .slice(pageStart, pageStart + OBSERVABILITY_PIPELINES_LIMIT)
      .map(({ pipeline, health }) => ({
        pipeline,
        health,
        cells: createObservabilityPipelinesCells(
          pipeline,
          health,
          timeframe,
          bucketStartMs,
          completedCounts,
          failedCounts,
        ),
      }));
  }, [entries, pageStart, timeframe, bucketStartMs, completedData, failedData]);

  const updatePage = (nextPage: number) => {
    void updateSearch(
      (prev) => ({
        ...prev,
        pipelinesPage: nextPage === OBSERVABILITY_DEFAULT_PIPELINES_PAGE ? undefined : nextPage,
      }),
      { replace: true },
    );
  };

  const handlePreviousPage = () => {
    updatePage(page - 1);
  };

  const handleNextPage = () => {
    updatePage(page + 1);
  };

  return (
    <Widget
      header="Pipelines"
      icon={FlowArrowIcon}
      isFlush
      gap={0}
      actions={
        <Button
          label="View all"
          size={ButtonSize.SMALL}
          variant={ButtonVariant.TERTIARY}
          icon={ArrowRightIcon}
          isIconTrailing
          href={createFilamentHref(FilamentPath.PIPELINES)}
          as={RouterLink}
        />
      }
    >
      {entries.length === 0 ? (
        <FlexItem grow={1} minHeight={0}>
          <ObservabilityWidgetEmpty icon={FlowArrowIcon} header="No pipelines" />
        </FlexItem>
      ) : (
        <ObservabilityPipelinesFrame
          $trackWidth={getObservabilityPipelinesTrackWidth(getObservabilityBucketCount(timeframe))}
        >
          <ObservabilityPipelinesHeader />
          <Box minHeight={hasPages ? OBSERVABILITY_PIPELINES_PAGE_MIN_HEIGHT : undefined}>
            {rows.map(({ pipeline, health, cells }) => (
              <ObservabilityPipelinesRow
                key={pipeline.id}
                pipeline={pipeline}
                health={health}
                cells={cells}
              />
            ))}
          </Box>
          {hasPages && (
            <Flex
              alignItems={AlignItems.CENTER}
              justifyContent={JustifyContent.SPACE_BETWEEN}
              gap={8}
              height={OBSERVABILITY_PIPELINES_FOOTER_HEIGHT}
              padding={[0, 12]}
            >
              <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
                {`${pageStart + 1}–${pageStart + rows.length} of ${entries.length}`}
              </Text>
              <Flex gap={4}>
                <Button
                  icon={CaretLeftIcon}
                  size={ButtonSize.SMALL}
                  variant={ButtonVariant.TERTIARY}
                  ariaLabel="Previous page"
                  tooltip="Previous page"
                  isDisabled={page === OBSERVABILITY_DEFAULT_PIPELINES_PAGE}
                  onClick={handlePreviousPage}
                />
                <Button
                  icon={CaretRightIcon}
                  size={ButtonSize.SMALL}
                  variant={ButtonVariant.TERTIARY}
                  ariaLabel="Next page"
                  tooltip="Next page"
                  isDisabled={page === pageCount}
                  onClick={handleNextPage}
                />
              </Flex>
            </Flex>
          )}
        </ObservabilityPipelinesFrame>
      )}
    </Widget>
  );
};

export default ObservabilityPipelinesWidget;
