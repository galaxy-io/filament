import type { FC } from "react";

import Square, { type SquareSize } from "@galaxy-io/dls/shapes/Square";

import type { RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { getPipelineRunStatusHue } from "@/components/runs/utils";

interface PipelineRunStatusSwatchProps {
  status: RunStatus;
  failedResources?: RunInfo["failedResources"];
  size?: SquareSize;
}

const PipelineRunStatusSwatch: FC<PipelineRunStatusSwatchProps> = ({
  status,
  failedResources,
  size,
}) => {
  return <Square size={size} color={getPipelineRunStatusHue(status, failedResources)} />;
};

export default PipelineRunStatusSwatch;
