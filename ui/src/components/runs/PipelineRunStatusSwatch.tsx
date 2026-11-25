import type { FC } from "react";

import Square, { type SquareSize } from "@galaxy-io/dls/shapes/Square";

import type { RunInfo, RunStatus } from "@/gen/ingestion/v1/runs_pb";

import { getPipelineRunStatusHue } from "@/components/runs/utils";

interface PipelineRunStatusSwatchProps {
  status: RunStatus;
  failedResources?: RunInfo["failedResources"];
  executionStatus?: RunInfo["executionStatus"];
  size?: SquareSize;
}

const PipelineRunStatusSwatch: FC<PipelineRunStatusSwatchProps> = ({
  status,
  failedResources,
  executionStatus,
  size,
}) => (
  <Square size={size} color={getPipelineRunStatusHue(status, failedResources, executionStatus)} />
);

export default PipelineRunStatusSwatch;
