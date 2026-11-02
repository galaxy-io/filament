import type { FC } from "react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

import KeyValueList from "@/components/KeyValueList";
import KeyValueListRow from "@/components/KeyValueListRow";
import PipelineRunRecords from "@/components/runs/PipelineRunRecords";
import PipelineRunStatus from "@/components/runs/PipelineRunStatus";
import PipelineRunVolume from "@/components/runs/PipelineRunVolume";

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
      <KeyValueListRow label="Records" value={<PipelineRunRecords run={run} />} />
      <KeyValueListRow label="Volume" value={<PipelineRunVolume run={run} />} />
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
