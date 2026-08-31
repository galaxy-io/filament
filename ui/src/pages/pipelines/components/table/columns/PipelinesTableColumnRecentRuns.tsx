import type { MouseEvent } from "react";
import { Fragment, useMemo } from "react";

import { create } from "@bufbuild/protobuf";
import { styled } from "@linaria/react";
import { useNavigate } from "@tanstack/react-router";

import {
  ChartTooltipGrid,
  ChartTooltipLabelCell,
  ChartTooltipValueCell,
} from "@galaxy-io/dls/charts/ChartPrimitives";
import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Wrapper from "@galaxy-io/dls/containers/Wrapper";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import TextShimmer from "@galaxy-io/dls/text/TextShimmer";
import { withTheme } from "@galaxy-io/dls/theme/GalaxyTheme";
import type { PropsWithTheme } from "@galaxy-io/dls/theme/types";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";

import { PaginationRequestSchema } from "@/gen/ingestion/v1/pagination_pb";
import { GetPipelineRequestSchema, type Pipeline } from "@/gen/ingestion/v1/pipelines_pb";
import { ListRunsRequestSchema, type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import {
  PIPELINES_TABLE_RECENT_RUNS_COUNT,
  PIPELINES_TABLE_RECENT_RUNS_STATUS_TO_COLOR_MAP,
  PIPELINES_TABLE_RECENT_RUNS_STATUSES,
} from "@/pages/pipelines/components/table/constants";
import { PIPELINE_RUN_STATUS_TO_LABEL_MAP } from "@/pages/pipelines/history/constants";
import { getPipelineHistoryRunTimestamp } from "@/pages/pipelines/history/utils";

import { useGetPipelineQuery } from "@/api/queries/pipelines";
import { useListRunsQuery } from "@/api/queries/runs";

import { formatBytes, formatCount, formatDuration, formatTimestamp } from "@/utils/format";

interface PipelinesTableColumnRecentRunsProps {
  pipeline: Pipeline;
}

const PipelinesTableRecentRunsWrapper = styled.div`
  display: flex;
  gap: 4px;

  & > * {
    transition: opacity 100ms ease;
  }

  &:hover > *:not(:hover):not([data-tippy-root]) {
    opacity: 0.4;
  }
`;

const PipelinesTableRecentRunSquare = withTheme(styled.div<
  PropsWithTheme<{ $status?: RunStatus; $isClickable?: boolean }>
>`
  width: 10px;
  height: 10px;
  border-radius: 2px;
  cursor: ${({ $isClickable }) => ($isClickable ? "pointer" : "default")};
  background: ${({ theme, $status = RunStatus.UNSPECIFIED }) =>
    PIPELINES_TABLE_RECENT_RUNS_STATUS_TO_COLOR_MAP[$status](theme)};
`);

const PipelinesTableRecentRunTooltip = ({ run }: { run: RunInfo }) => {
  const rows = [
    { label: "Duration", value: formatDuration(run.startedAt, run.endedAt) },
    { label: "Records", value: formatCount(run.records) },
    { label: "Volume", value: formatBytes(run.bytes) },
  ];

  return (
    <Wrapper minWidth={160}>
      <FlexWrapper direction={FlexDirection.COLUMN} gap={8} fillWidth>
        <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} isEllipsis>
          {formatTimestamp(getPipelineHistoryRunTimestamp(run).timestamp)}
        </Text>
        {run.error ? (
          <Text size={TextSize.CAPTION} isMonospace isSelectable>
            {run.error}
          </Text>
        ) : (
          <ChartTooltipGrid>
            {rows.map((row) => (
              <Fragment key={row.label}>
                <ChartTooltipLabelCell>
                  <Text size={TextSize.BODY_SM} isEllipsis>
                    {row.label}
                  </Text>
                </ChartTooltipLabelCell>
                <ChartTooltipValueCell>
                  <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM} align="right">
                    {row.value}
                  </Text>
                </ChartTooltipValueCell>
              </Fragment>
            ))}
          </ChartTooltipGrid>
        )}
      </FlexWrapper>
    </Wrapper>
  );
};

const PipelinesTableColumnRecentRuns = ({ pipeline }: PipelinesTableColumnRecentRunsProps) => {
  const navigate = useNavigate();
  const { data, isLoading } = useListRunsQuery({
    input: create(ListRunsRequestSchema, {
      pipelineId: pipeline.id,
      status: PIPELINES_TABLE_RECENT_RUNS_STATUSES,
      pagination: create(PaginationRequestSchema, {
        pageSize: PIPELINES_TABLE_RECENT_RUNS_COUNT,
      }),
    }),
  });
  const { data: pipelineData } = useGetPipelineQuery({
    input: create(GetPipelineRequestSchema, { id: pipeline.id, includeSchedule: true }),
    options: { enabled: Boolean(pipeline.id) },
  });

  const runs = useMemo(() => [...(data?.runs ?? [])].reverse(), [data]);
  const schedule = pipelineData?.pipeline?.schedule;
  const hasScheduledRun = Boolean(schedule?.config?.isEnabled && schedule.nextFireAt);
  const visibleRuns = hasScheduledRun ? runs.slice(-(PIPELINES_TABLE_RECENT_RUNS_COUNT - 1)) : runs;
  const scheduledIndex = hasScheduledRun ? visibleRuns.length : -1;

  const handleRunClick = (event: MouseEvent, runId: RunInfo["id"]) => {
    event.stopPropagation();
    navigate({
      to: "/pipelines/$id/history",
      params: { id: pipeline.id },
      search: { runId: [runId] },
    });
  };

  if (isLoading) {
    return <TextShimmer width={136} height={18} />;
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
              isInteractive={Boolean(run.error)}
            >
              <PipelinesTableRecentRunSquare
                $status={run.status}
                $isClickable
                onClick={(event) => handleRunClick(event, run.id)}
              />
            </Tooltip>
          );
        }
        if (index === scheduledIndex && schedule) {
          return (
            <Tooltip key="scheduled" body={`Scheduled for ${formatTimestamp(schedule.nextFireAt)}`}>
              <PipelinesTableRecentRunSquare $status={RunStatus.SCHEDULED} />
            </Tooltip>
          );
        }
        // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder slots with no identity
        return <PipelinesTableRecentRunSquare key={`empty-${index}`} />;
      })}
    </PipelinesTableRecentRunsWrapper>
  );
};

export default PipelinesTableColumnRecentRuns;
