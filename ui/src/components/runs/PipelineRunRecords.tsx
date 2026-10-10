import type { FC } from "react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { EMPTY_VALUE, formatNumber } from "@galaxy-io/dls/utils/format";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

interface PipelineRunRecordsProps {
  run: RunInfo;
}

const PipelineRunRecords: FC<PipelineRunRecordsProps> = ({ run }) => (
  <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
    {run.startedAt ? formatNumber(run.records) : EMPTY_VALUE}
  </Text>
);

export default PipelineRunRecords;
