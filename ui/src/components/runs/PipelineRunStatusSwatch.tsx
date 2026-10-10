import type { FC } from "react";

import Square, { type SquareSize } from "@galaxy-io/dls/shapes/Square";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { PIPELINE_RUN_STATUS_TO_HUE_MAP } from "@/components/runs/constants";

interface PipelineRunStatusSwatchProps {
  status: RunStatus;
  size?: SquareSize;
}

const PipelineRunStatusSwatch: FC<PipelineRunStatusSwatchProps> = ({ status, size }) => {
  const hue = PIPELINE_RUN_STATUS_TO_HUE_MAP[status];
  return <Square size={size} {...(hue === null ? {} : { color: hue })} />;
};

export default PipelineRunStatusSwatch;
