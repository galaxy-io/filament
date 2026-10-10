import type { FC } from "react";

import Skeleton from "@galaxy-io/dls/feedback/Skeleton";
import Box from "@galaxy-io/dls/layout/Box";

import { PIPELINE_CANVAS_NODE_TABLE_LIST_SHIMMER_COUNT } from "@/pages/pipelines/canvas/nodes/constants";

const PipelineCanvasNodeSourceIslandTableShimmer: FC = () => (
  <>
    {Array.from({ length: PIPELINE_CANVAS_NODE_TABLE_LIST_SHIMMER_COUNT }).map((_, index) => (
      // biome-ignore lint/suspicious/noArrayIndexKey: static placeholder rows with no identity
      <Box key={index} width="100%">
        <Skeleton />
      </Box>
    ))}
  </>
);

export default PipelineCanvasNodeSourceIslandTableShimmer;
