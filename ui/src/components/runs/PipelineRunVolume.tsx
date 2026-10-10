import type { FC } from "react";

import Text, { TextSize } from "@galaxy-io/dls/text/Text";
import { FontFamily } from "@galaxy-io/dls/theme/enums";
import { EMPTY_VALUE, formatBytes } from "@galaxy-io/dls/utils/format";

import type { RunInfo } from "@/gen/ingestion/v1/runs_pb";

interface PipelineRunVolumeProps {
  run: RunInfo;
}

const PipelineRunVolume: FC<PipelineRunVolumeProps> = ({ run }) => (
  <Text size={TextSize.BODY_SM} family={FontFamily.MONO}>
    {run.startedAt ? formatBytes(run.bytes) : EMPTY_VALUE}
  </Text>
);

export default PipelineRunVolume;
