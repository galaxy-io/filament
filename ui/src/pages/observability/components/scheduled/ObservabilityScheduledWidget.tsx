import { type FC, useMemo } from "react";

import { styled } from "@linaria/react";
import { CalendarIcon } from "@phosphor-icons/react";

import Skeleton, { SkeletonSize } from "@galaxy-io/dls/feedback/Skeleton";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import FlexItem from "@galaxy-io/dls/layout/FlexItem";
import ScrollArea from "@galaxy-io/dls/layout/ScrollArea";
import { FOCUS_RING, HAIRLINE_WIDTH } from "@galaxy-io/dls/styles/mixins";
import Text, { TextSize, TextVariant } from "@galaxy-io/dls/text/Text";
import { t } from "@galaxy-io/dls/theme/tokens/t";
import Tooltip from "@galaxy-io/dls/tooltip/Tooltip";
import { formatRelativeTime } from "@galaxy-io/dls/utils/format";
import Widget from "@galaxy-io/dls/widget/Widget";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import PipelineName from "@/components/pipelines/PipelineName";
import RouterLink from "@/components/RouterLink";
import PipelineRunStatusSwatch from "@/components/runs/PipelineRunStatusSwatch";

import ObservabilityWidgetEmpty from "@/pages/observability/components/ObservabilityWidgetEmpty";
import { OBSERVABILITY_RUNS_SCHEDULED_INPUT } from "@/pages/observability/components/runs/constants";
import { OBSERVABILITY_LIST_ROW_HEIGHT } from "@/pages/observability/constants";
import { useObservabilityPipelines } from "@/pages/observability/hooks/useObservabilityPipelines";

import { createFilamentHref, FilamentPath } from "@/module/paths";

import { useListRunsQuery } from "@/api/queries/runs";

import { formatTimestamp } from "@/utils/format";

const ObservabilityScheduledRow = styled(RouterLink)`
  ${FOCUS_RING}
  display: flex;
  align-items: center;
  gap: ${t.space[8]};
  height: ${OBSERVABILITY_LIST_ROW_HEIGHT}px;
  padding: 0 ${t.space[12]};
  color: inherit;
  text-decoration: none;
  border-bottom: ${HAIRLINE_WIDTH} solid ${t.color.border.primary};
  transition: background-color ${t.duration.fast};

  &:hover {
    background-color: ${t.color.background.hovered};
  }
`;

const ObservabilityScheduledWidget: FC = () => {
  const { pipelinesById } = useObservabilityPipelines();
  const { data, isLoading } = useListRunsQuery({ input: OBSERVABILITY_RUNS_SCHEDULED_INPUT });

  const runs = useMemo(() => {
    const pipelineIds = new Set<string>();
    return (data?.runs ?? []).filter((run) => {
      if (pipelineIds.has(run.pipelineId)) {
        return false;
      }
      pipelineIds.add(run.pipelineId);
      return true;
    });
  }, [data]);

  const renderRows = () => {
    if (isLoading) {
      return (
        <Box padding={12}>
          <Skeleton size={SkeletonSize.SMALL} />
        </Box>
      );
    }
    return runs.map((run) => (
      <ObservabilityScheduledRow
        key={run.id}
        href={createFilamentHref(FilamentPath.PIPELINE, { id: run.pipelineId })}
      >
        <PipelineRunStatusSwatch status={RunStatus.SCHEDULED} />
        <FlexItem grow={1} minWidth={0}>
          <PipelineName pipelineId={run.pipelineId} pipeline={pipelinesById.get(run.pipelineId)} />
        </FlexItem>
        <FlexItem shrink={0}>
          <Tooltip body={formatTimestamp(run.scheduledAt)}>
            <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
              {formatRelativeTime(run.scheduledAt)}
            </Text>
          </Tooltip>
        </FlexItem>
      </ObservabilityScheduledRow>
    ));
  };

  return (
    <Widget header="Scheduled" icon={CalendarIcon} isFlush gap={0}>
      <FlexItem grow={1} basis={0} minHeight={0}>
        {!isLoading && runs.length === 0 ? (
          <ObservabilityWidgetEmpty icon={CalendarIcon} header="No scheduled runs" />
        ) : (
          <ScrollArea>
            <Box variant={BoxVariant.PRIMARY} fillWidth>
              {renderRows()}
            </Box>
          </ScrollArea>
        )}
      </FlexItem>
    </Widget>
  );
};

export default ObservabilityScheduledWidget;
