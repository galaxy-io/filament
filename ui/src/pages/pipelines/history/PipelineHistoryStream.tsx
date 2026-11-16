import type { FC } from "react";

import { WebhooksLogoIcon } from "@phosphor-icons/react";
import pluralize from "pluralize";

import Icon, { IconVariant } from "@galaxy-io/dls/icons/Icon";
import Box, { BoxVariant } from "@galaxy-io/dls/layout/Box";
import Flex, { AlignItems, JustifyContent } from "@galaxy-io/dls/layout/Flex";
import Text, { TextSize, TextVariant, TextWeight } from "@galaxy-io/dls/text/Text";
import { Radius } from "@galaxy-io/dls/theme/enums";
import { formatNumber, formatRelativeTime } from "@galaxy-io/dls/utils/format";

import { ExecutionMode } from "@/gen/ingestion/v1/common_pb";
import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import PipelineRunStatus from "@/components/runs/PipelineRunStatus";

import PipelinePageActiveRunActions from "@/pages/pipelines/components/header/PipelinePageActiveRunActions";
import PipelinePageStreamButton from "@/pages/pipelines/components/header/PipelinePageStreamButton";
import { usePipelineExecutionLane } from "@/pages/pipelines/hooks/usePipelineExecutionLane";
import { usePipelineRun } from "@/pages/pipelines/hooks/usePipelineRun";

import { usePipelineParams } from "@/module/hooks";

import { createGetPipelineInput, useSuspenseGetPipelineQuery } from "@/api/queries/pipelines";
import { useListActivePipelineRunsQuery } from "@/api/queries/runs";

import { isContinuousRunActive } from "@/utils/runs";

interface PipelineHistoryStreamProps {
  lastStreamRun: RunInfo | undefined;
}

const PipelineHistoryStream: FC<PipelineHistoryStreamProps> = ({ lastStreamRun }) => {
  const { id } = usePipelineParams();
  const { data } = useSuspenseGetPipelineQuery({ input: createGetPipelineInput(id) });
  const { data: activeRunsData } = useListActivePipelineRunsQuery(id);
  const { hasStreamResources } = usePipelineExecutionLane();
  const { startRun, isRunning } = usePipelineRun();

  const activeStream = activeRunsData?.runs.find(isContinuousRunActive);
  const run = activeStream ?? lastStreamRun;
  const pipeline = data.pipeline;

  const actions = activeStream ? (
    <PipelinePageActiveRunActions run={activeStream} isStream isCompact />
  ) : (
    <PipelinePageStreamButton
      runErrors={[]}
      isRunnable={hasStreamResources && !!pipeline}
      isStarting={isRunning}
      onClick={() => {
        if (pipeline) startRun(pipeline, { executionMode: ExecutionMode.CONTINUOUS });
      }}
      isCompact
    />
  );

  const lastCommittedAt = run?.executionStatus?.lastCommittedAt;
  const startedAt = run?.startedAt;

  return (
    <Box padding={[8, 12]}>
      <Box variant={BoxVariant.PRIMARY} radius={Radius.MD} padding={[4, 4, 4, 12]} hasBorder>
        <Flex alignItems={AlignItems.CENTER} justifyContent={JustifyContent.SPACE_BETWEEN} gap={12}>
          <Flex alignItems={AlignItems.CENTER} gap={12} minWidth={0}>
            <Flex alignItems={AlignItems.CENTER} gap={8} shrink={0}>
              <Icon component={WebhooksLogoIcon} variant={IconVariant.SECONDARY} />
              <Text size={TextSize.BODY_SM} weight={TextWeight.MEDIUM}>
                Stream
              </Text>
            </Flex>
            {run ? (
              <>
                <PipelineRunStatus
                  status={run.status}
                  executionStatus={run.executionStatus}
                  error={run.error}
                />
                {startedAt ? (
                  <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
                    {`${activeStream ? "Started" : "Last ran"} ${formatRelativeTime(startedAt)}`}
                  </Text>
                ) : null}
                <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
                  {`${formatNumber(run.records)} ${pluralize("record", Number(run.records))}`}
                </Text>
                {lastCommittedAt ? (
                  <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY} lineClamp={1}>
                    {`Committed ${formatRelativeTime(lastCommittedAt)}`}
                  </Text>
                ) : null}
              </>
            ) : (
              <Text size={TextSize.BODY_SM} variant={TextVariant.TERTIARY}>
                Not started
              </Text>
            )}
          </Flex>
          <Flex alignItems={AlignItems.CENTER} gap={8} shrink={0}>
            {actions}
          </Flex>
        </Flex>
      </Box>
    </Box>
  );
};

export default PipelineHistoryStream;
