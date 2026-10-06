import type { SquareSize } from "@galaxy-io/dls/shapes/Square";

import type { RunStatus } from "@/gen/ingestion/v1/runs_pb";

import HueSquare from "@/components/HueSquare";

import { PIPELINE_RUN_STATUS_TO_HUE_MAP } from "@/pages/pipelines/history/constants";

interface PipelineRunStatusSwatchProps {
  status: RunStatus;
  size?: SquareSize;
}

const PipelineRunStatusSwatch = ({ status, size }: PipelineRunStatusSwatchProps) => (
  <HueSquare hue={PIPELINE_RUN_STATUS_TO_HUE_MAP[status]} size={size} />
);

export default PipelineRunStatusSwatch;
