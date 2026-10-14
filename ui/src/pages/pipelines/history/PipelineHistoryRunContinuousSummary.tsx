import type { FC } from "react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { formatBytes, formatNumber } from "@galaxy-io/dls/utils/format";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import KeyValueList from "@/components/KeyValueList";
import KeyValueListRow from "@/components/KeyValueListRow";
import PipelineRunStatus from "@/components/runs/PipelineRunStatus";

import { formatTimestamp } from "@/utils/format";

interface PipelineHistoryRunContinuousSummaryProps {
  run: RunInfo;
}

const PipelineHistoryRunContinuousSummary: FC<PipelineHistoryRunContinuousSummaryProps> = ({
  run,
}) => {
  const lastCommittedAt = run.executionStatus?.lastCommittedAt;
  return (
    <KeyValueList hasBorder={false}>
      <KeyValueListRow
        label="Status"
        value={
          <PipelineRunStatus
            status={run.status}
            executionStatus={run.executionStatus}
            error={run.error}
          />
        }
      />
      <KeyValueListRow
        label="Committed"
        value={
          <Text size={TextSize.BODY_SM}>
            {formatNumber(run.records)} records · {formatBytes(run.bytes)}
          </Text>
        }
      />
      <KeyValueListRow
        label="Last commit"
        value={
          <Text size={TextSize.BODY_SM}>
            {lastCommittedAt ? formatTimestamp(lastCommittedAt) : "Waiting for the first commit"}
          </Text>
        }
      />
    </KeyValueList>
  );
};

export default PipelineHistoryRunContinuousSummary;
