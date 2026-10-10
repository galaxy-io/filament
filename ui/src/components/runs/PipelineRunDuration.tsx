import { type FC, useEffect, useState } from "react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { type RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { PIPELINE_RUN_DURATION_TICK_MS } from "@/components/runs/constants";

import { formatRunDuration } from "@/utils/runs";

interface PipelineRunDurationProps {
  run: RunInfo;
}

const PipelineRunDuration: FC<PipelineRunDurationProps> = ({ run }) => {
  const isLive = run.status === RunStatus.RUNNING && Boolean(run.startedAt) && !run.endedAt;
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!isLive) return;
    const interval = setInterval(() => setNow(Date.now()), PIPELINE_RUN_DURATION_TICK_MS);
    return () => clearInterval(interval);
  }, [isLive]);

  return (
    <Text size={TextSize.BODY_SM} lineClamp={1}>
      {formatRunDuration(run.startedAt, isLive ? BigInt(now) : run.endedAt)}
    </Text>
  );
};

export default PipelineRunDuration;
