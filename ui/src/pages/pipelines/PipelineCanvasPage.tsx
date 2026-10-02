import { styled } from "@linaria/react";
import { match } from "ts-pattern";

import { usePipelineCanvasSelection } from "@/pages/pipelines/canvas/hooks/usePipelineCanvasSelection";
import PipelineCanvasFlow from "@/pages/pipelines/canvas/PipelineCanvasFlow";
import PipelineCanvasRoutes from "@/pages/pipelines/canvas/routes/PipelineCanvasRoutes";
import { PipelineCanvasView } from "@/pages/pipelines/canvas/types";

const PipelineCanvasPageWrapper = styled.div`
  position: relative;
  flex: 1;
  min-width: 0;
  height: 100%;
`;

const PipelineCanvasPage = () => {
  const { view } = usePipelineCanvasSelection();

  return (
    <PipelineCanvasPageWrapper>
      {match(view)
        .with(PipelineCanvasView.ROUTES, () => <PipelineCanvasRoutes />)
        .with(PipelineCanvasView.CANVAS, () => <PipelineCanvasFlow />)
        .exhaustive()}
    </PipelineCanvasPageWrapper>
  );
};

export default PipelineCanvasPage;
