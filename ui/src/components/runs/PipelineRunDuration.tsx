import type { FC } from "react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { usePipelineRunNow } from "@/components/runs/hooks/usePipelineRunNow";

import { formatRunDuration } from "@/utils/runs";

interface PipelineRunDurationProps {
  run: RunInfo;
}

const PipelineRunDuration: FC<PipelineRunDurationProps> = ({ run }) => {
  const isLive = run.status === RunStatus.RUNNING && Boolean(run.startedAt) && !run.endedAt;
  const now = usePipelineRunNow(isLive);

  return (
    <Text size={TextSize.BODY_SM} lineClamp={1}>
      {formatRunDuration(run.startedAt, isLive ? BigInt(now) : run.endedAt)}
    </Text>
  );
};

export default PipelineRunDuration;
