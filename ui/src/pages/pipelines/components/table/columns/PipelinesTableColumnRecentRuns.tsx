import type { FC, MouseEvent } from "react";
import { Fragment, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";

import Skeleton, { SkeletonSize } from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, FlexDirection } from "@galaxy-io/dls/layout/Flex";
import Grid, { GridAlignItems } from "@galaxy-io/dls/layout/Grid";
import Square, { SquareSize, SquareVariant } from "@galaxy-io/dls/shapes/Square";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import { formatBytes, formatNumber } from "@galaxy-io/dls/utils/format";

import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import type { Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { ListRunsRequestSchema, type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { getPipelineNextFireAt } from "@/components/pipelines/utils";
import {
  PIPELINE_RUN_EXECUTED_STATUSES,
  PIPELINE_RUN_STATUS_TO_LABEL_MAP,
} from "@/components/runs/constants";
import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

import { PIPELINES_TABLE_RECENT_RUNS_COUNT } from "@/pages/pipelines/components/table/constants";
import { getPipelineHistoryRunTimestamp } from "@/pages/pipelines/history/utils";

import { useFilamentNavigate } from "@/module/hooks";
import { FilamentPath } from "@/module/paths";

import { useListRunsQuery } from "@/api/queries/runs";

import { formatTimestamp } from "@/utils/format";
import { formatRunDuration } from "@/utils/runs";

interface PipelinesTableColumnRecentRunsProps {
  pipeline: Pipeline;
}

const PipelinesTableRecentRunsWrapper = styled.div`
  display: flex;
  gap: ${t.space[4]};

  & > * {
    transition: opacity 100ms ease;
  }

  &:hover > *:not(:hover):not([data-tippy-root]) {
    opacity: 0.4;
  }
`;

const PipelinesTableRecentRunTooltip: FC<{ run: RunInfo }> = ({ run }) => {
  const rows = [
    { label: "Duration", value: formatRunDuration(run.startedAt, run.endedAt) },
    { label: "Records", value: formatNumber(run.records) },
    { label: "Volume", value: formatBytes(run.bytes) },
  ];

  return (
    <Box minWidth={160}>
      <Flex alignItems={AlignItems.START} direction={FlexDirection.COLUMN} gap={8} fillWidth>
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
          {formatTimestamp(getPipelineHistoryRunTimestamp(run).timestamp)}
        </Text>
        {run.error ? (
          <Text size={TextSize.CAPTION} family={FontFamily.MONO} isSelectable>
            {run.error}
          </Text>
        ) : (
          <Grid
            columns="minmax(0, 1fr) auto"
            gap={[4, 16]}
            alignItems={GridAlignItems.CENTER}
            fillWidth
          >
            {rows.map((row) => (
              <Fragment key={row.label}>
                <Text size={TextSize.BODY_SM} lineClamp={1}>
                  {row.label}
                </Text>
                <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} align="right">
                  {row.value}
                </Text>
              </Fragment>
            ))}
          </Grid>
        )}
      </Flex>
    </Box>
  );
};

const PipelinesTableColumnRecentRuns: FC<PipelinesTableColumnRecentRunsProps> = ({ pipeline }) => {
  const navigate = useFilamentNavigate();
  const { data, isLoading } = useListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: pipeline.id,
      status: PIPELINE_RUN_EXECUTED_STATUSES,
      pagination: create(PaginationRequestSchema, {
        pageSize: PIPELINES_TABLE_RECENT_RUNS_COUNT,
      }),
    }),
  });

  const runs = useMemo(() => [...(data?.runs ?? [])].reverse(), [data]);
  const nextFireAt = getPipelineNextFireAt(pipeline);
  const hasScheduledRun = nextFireAt !== undefined;
  const visibleRuns = hasScheduledRun ? runs.slice(-(PIPELINES_TABLE_RECENT_RUNS_COUNT - 1)) : runs;
  const scheduledIndex = hasScheduledRun ? visibleRuns.length : -1;

  const handleRunClick = (event: MouseEvent, runId: RunInfo["id"]) => {
    event.stopPropagation();
    void navigate({
      to: FilamentPath.PIPELINE_HISTORY,
      params: { id: pipeline.id },
      search: { runId: [runId] },
    });
  };

  if (isLoading) {
    return (
      <Box width={136}>
        <Skeleton size={SkeletonSize.X_SMALL} />
      </Box>
    );
  }

  return (
    <PipelinesTableRecentRunsWrapper>
      {Array.from({ length: PIPELINES_TABLE_RECENT_RUNS_COUNT }, (_, index) => {
        const run = visibleRuns[index];
        if (run) {
          return (
            <Tooltip
              key={run.id}
              body={
                run.endedAt ? (
                  <PipelinesTableRecentRunTooltip run={run} />
                ) : (
                  PIPELINE_RUN_STATUS_TO_LABEL_MAP[run.status]
                )
              }
            >
              <Flex onClick={(event) => handleRunClick(event, run.id)}>
                <PipelineRunStatusSwatch status={run.status} size={SquareSize.MEDIUM} />
              </Flex>
            </Tooltip>
          );
        }
        if (index === scheduledIndex && nextFireAt) {
          return (
            <Tooltip key="scheduled" body={`Scheduled for ${formatTimestamp(nextFireAt)}`}>
              <Flex>
                <PipelineRunStatusSwatch status={RunStatus.SCHEDULED} size={SquareSize.MEDIUM} />
              </Flex>
            </Tooltip>
          );
        }
        return (
          // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder slots with no identity
          <Flex key={`empty-${index}`}>
            <Square size={SquareSize.MEDIUM} variant={SquareVariant.DISABLED} />
          </Flex>
        );
      })}
    </PipelinesTableRecentRunsWrapper>
  );
};

export default PipelinesTableColumnRecentRuns;
