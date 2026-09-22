import FlexWrapper, { FlexDirection } from "@galaxy-io/dls/containers/FlexWrapper";
import Text from "@galaxy-io/dls/text/Text";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import { formatBytes, formatCount, formatTimestamp } from "@/utils/format";

export default function ContinuousRunSummary({ run }: { run: RunInfo }) {
  const lastCommit = run.executionStatus?.lastCommittedAt;
  return (
    <FlexWrapper direction={FlexDirection.COLUMN} gap={8} padding={12} fillWidth>
      <PipelineHistoryRunStatus
        status={run.status}
        executionStatus={run.executionStatus}
        error={run.error}
      />
      <Text>
        {formatCount(run.records)} records committed · {formatBytes(run.bytes)}
      </Text>
      <Text>
        {lastCommit
          ? `Last commit: ${formatTimestamp(lastCommit)}`
          : "Waiting for the first commit"}
      </Text>
      {run.executionStatus?.reason && <Text>{run.executionStatus.reason}</Text>}
    </FlexWrapper>
  );
}
