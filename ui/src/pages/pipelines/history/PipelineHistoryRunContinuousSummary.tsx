import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import ConnectionDrawerKeyValueRow from "@/pages/connectors/components/drawer/ConnectionDrawerKeyValueRow";
import ConnectionDrawerList from "@/pages/connectors/components/drawer/ConnectionDrawerList";
import PipelineHistoryRunStatus from "@/pages/pipelines/history/PipelineHistoryRunStatus";

import { formatBytes, formatCount, formatTimestamp } from "@/utils/format";

interface PipelineHistoryRunContinuousSummaryProps {
  run: RunInfo;
}

const PipelineHistoryRunContinuousSummary = ({ run }: PipelineHistoryRunContinuousSummaryProps) => {
  const lastCommittedAt = run.executionStatus?.lastCommittedAt;
  return (
    <ConnectionDrawerList>
      <ConnectionDrawerKeyValueRow
        label="Status"
        value={
          <PipelineHistoryRunStatus
            status={run.status}
            executionStatus={run.executionStatus}
            error={run.error}
          />
        }
      />
      <ConnectionDrawerKeyValueRow
        label="Committed"
        value={
          <Text size={TextSize.BODY_SM}>
            {formatCount(run.records)} records · {formatBytes(run.bytes)}
          </Text>
        }
      />
      <ConnectionDrawerKeyValueRow
        label="Last commit"
        value={
          <Text size={TextSize.BODY_SM}>
            {lastCommittedAt ? formatTimestamp(lastCommittedAt) : "Waiting for the first commit"}
          </Text>
        }
      />
    </ConnectionDrawerList>
  );
};

export default PipelineHistoryRunContinuousSummary;
