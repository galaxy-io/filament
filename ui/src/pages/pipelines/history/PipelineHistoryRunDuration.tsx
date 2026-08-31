import { useEffect, useState } from "react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { formatDuration } from "@/utils/format";

const PIPELINE_HISTORY_RUN_DURATION_TICK_MS = 1_000;

interface PipelineHistoryRunDurationProps {
  status: RunStatus;
  startedAt: bigint;
  endedAt: bigint;
}

const PipelineHistoryRunDuration = ({
  status,
  startedAt,
  endedAt,
}: PipelineHistoryRunDurationProps) => {
  const isLive = status === RunStatus.RUNNING && Boolean(startedAt) && !endedAt;
  const [now, setNow] = useState(() => Date.now());

  useEffect(() => {
    if (!isLive) return;
    const interval = setInterval(() => setNow(Date.now()), PIPELINE_HISTORY_RUN_DURATION_TICK_MS);
    return () => clearInterval(interval);
  }, [isLive]);

  return (
    <Text size={TextSize.BODY_SM} isEllipsis>
      {formatDuration(startedAt, isLive ? BigInt(now) : endedAt)}
    </Text>
  );
};

export default PipelineHistoryRunDuration;
